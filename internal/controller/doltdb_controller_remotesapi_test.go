// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("DoltDB RemotesAPI and EventScheduler", Ordered, func() {
	var (
		remotesKey = types.NamespacedName{
			Name:      "dolt-remotesapi",
			Namespace: "default",
		}

		remotesCredsKey = types.NamespacedName{
			Name:      "dolt-remotesapi-credentials",
			Namespace: remotesKey.Namespace,
		}

		doltdbRemotes *doltv1alpha.DoltDB
	)

	newDoltDB := func(name string, server doltv1alpha.Server) *doltv1alpha.DoltDB {
		return &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: remotesKey.Namespace,
			},
			Spec: doltv1alpha.DoltDBSpec{
				Image:         "dolthub/dolt",
				EngineVersion: doltdbEngineVersion,
				Replicas:      1,
				Storage: doltv1alpha.Storage{
					Size:             ptr.To(resource.MustParse("1Gi")),
					StorageClassName: ptr.To("standard-resize"),
					RetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
						WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
						WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					},
				},
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("500m"),
						"memory": resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						"memory": resource.MustParse("1Gi"),
					},
				},
				Probes: doltv1alpha.Probes{
					LivenessProbe: &corev1.Probe{
						InitialDelaySeconds: 20,
						PeriodSeconds:       10,
						TimeoutSeconds:      5,
					},
					ReadinessProbe: &corev1.Probe{
						InitialDelaySeconds: 15,
						PeriodSeconds:       10,
						TimeoutSeconds:      5,
					},
				},
				Server: server,
			},
		}
	}

	BeforeAll(func() {
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      remotesCredsKey.Name,
				Namespace: remotesCredsKey.Namespace,
			},
			Type: "Opaque",
			StringData: map[string]string{
				"admin-user":     "root",
				"admin-password": "12345",
			},
		}

		if err := k8sClient.Delete(ctx, &secret); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning remotesapi test secret")
			}
		}
		Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

		doltdbRemotes = newDoltDB(remotesKey.Name, doltv1alpha.Server{
			Behavior: doltv1alpha.Behavior{
				EventScheduler: "OFF",
			},
			RemotesAPI: &doltv1alpha.ServerRemotesAPI{},
		})
		// dolt_clone reads the password for --user from the server's environment.
		doltdbRemotes.Spec.Env = []corev1.EnvVar{
			{
				Name: "DOLT_REMOTE_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: ptr.To(doltdbRemotes.RootPasswordSecretKeyRef().ToKubernetesType()),
				},
			},
		}

		if err := k8sClient.Delete(ctx, doltdbRemotes); err != nil {
			if err != client.IgnoreNotFound(err) {
				log.FromContext(ctx).Error(err, "error cleaning remotesapi test doltdb")
			}
		}

		By("Creating standalone DoltDB with the remotes API and the event scheduler off")
		Expect(k8sClient.Create(ctx, doltdbRemotes)).To(Succeed())
		DeferCleanup(func() {
			deleteDoltDB(ctx, remotesKey, remotesCredsKey)
		})
	})

	It("should reject a remotes API port that collides with another server port", func() {
		tests := []struct {
			name   string
			server doltv1alpha.Server
		}{
			{
				name: "dolt-remotesapi-listener-collision",
				server: doltv1alpha.Server{
					RemotesAPI: &doltv1alpha.ServerRemotesAPI{Port: 3306},
				},
			},
			{
				name: "dolt-remotesapi-cluster-collision",
				server: doltv1alpha.Server{
					RemotesAPI: &doltv1alpha.ServerRemotesAPI{Port: 50051},
				},
			},
		}
		for _, tt := range tests {
			By(fmt.Sprintf("Creating DoltDB %s", tt.name))
			err := k8sClient.Create(ctx, newDoltDB(tt.name, tt.server))
			Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid error, got %v", err)
		}
	})

	It("should reject an unknown event scheduler value", func() {
		err := k8sClient.Create(ctx, newDoltDB("dolt-remotesapi-bad-scheduler", doltv1alpha.Server{
			Behavior: doltv1alpha.Behavior{EventScheduler: "MAYBE"},
		}))
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid error, got %v", err)
	})

	It("should reconcile the remotes API and event scheduler", func() {
		By("Expecting DoltDB to be ready eventually")
		expectReady(ctx, k8sClient, remotesKey)
		Expect(k8sClient.Get(ctx, remotesKey, doltdbRemotes)).To(Succeed())

		By("Expecting the remotes API port to be defaulted")
		Expect(doltdbRemotes.Spec.Server.RemotesAPI).NotTo(BeNil())
		Expect(doltdbRemotes.Spec.Server.RemotesAPI.Port).To(Equal(int32(50052)))

		By("Expecting ConfigMap with remotesapi and event_scheduler")
		Eventually(func(g Gomega) bool {
			var cm corev1.ConfigMap
			if err := k8sClient.Get(ctx, doltdbRemotes.DefaultConfigMapKey(), &cm); err != nil {
				return false
			}
			data, err := dolt.GenerateConfigMapData(doltdbRemotes)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cm.Data).To(Equal(data))
			for _, v := range cm.Data {
				var config dolt.Config
				g.Expect(yaml.Unmarshal([]byte(v), &config)).To(Succeed())
				g.Expect(config.RemotesAPI).To(Equal(&dolt.UserRemotesAPI{Port: 50052}))
				g.Expect(config.Behavior.EventScheduler).To(Equal("OFF"))
			}
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting the standalone Service to expose the remotes API port")
		Eventually(func(g Gomega) bool {
			var svc corev1.Service
			if err := k8sClient.Get(ctx, doltdbRemotes.ServiceKey(), &svc); err != nil {
				return false
			}
			port := findServicePort(svc.Spec.Ports, builder.DoltUserRemotesAPIPortName)
			g.Expect(port).NotTo(BeNil())
			g.Expect(port.Port).To(Equal(int32(50052)))
			return true
		}, testTimeout, testInterval).Should(BeTrue())

		By("Expecting the StatefulSet container to expose the remotes API port")
		var sts appsv1.StatefulSet
		Expect(k8sClient.Get(ctx, remotesKey, &sts)).To(Succeed())
		Expect(sts.Spec.Template.Spec.Containers[0].Ports).To(ContainElement(corev1.ContainerPort{
			Name:          builder.DoltUserRemotesAPIPortName,
			ContainerPort: 50052,
			Protocol:      corev1.ProtocolTCP,
		}))
	})

	It("should turn the event scheduler off", func() {
		sqlClient, err := sql.NewClientWithDoltDB(ctx, doltdbRemotes, refResolver)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(sqlClient.Close()).To(Succeed())
		}()

		var status string
		Expect(sqlClient.QueryRow(ctx, "SELECT @@global.event_scheduler").Scan(&status)).To(Succeed())
		Expect(status).To(Equal("OFF"))
	})

	It("should serve clones through the remotes API", func() {
		remoteURL := fmt.Sprintf(
			"http://%s:%d/remotes_src",
			statefulset.ServiceFQDNWithService(doltdbRemotes.ObjectMeta, doltdbRemotes.ServiceKey().Name),
			doltdbRemotes.Spec.Server.RemotesAPI.Port,
		)

		By("Creating a source database with a commit")
		sqlClient, err := sql.NewClientWithDoltDB(ctx, doltdbRemotes, refResolver)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(sqlClient.Close()).To(Succeed())
		}()
		Expect(sqlClient.Exec(ctx, "CREATE DATABASE IF NOT EXISTS remotes_src")).To(Succeed())

		srcClient, err := sql.NewClientWithDoltDB(ctx, doltdbRemotes, refResolver, sql.WithDatabase("remotes_src"))
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			Expect(srcClient.Close()).To(Succeed())
		}()
		Expect(srcClient.Exec(ctx, "CREATE TABLE t (id INT PRIMARY KEY)")).To(Succeed())
		Expect(srcClient.Exec(ctx, "INSERT INTO t VALUES (1), (2), (3)")).To(Succeed())
		Expect(srcClient.Exec(ctx, "CALL DOLT_COMMIT('-Am', 'seed')")).To(Succeed())

		By("Expecting a clone without credentials to be refused")
		err = sqlClient.Exec(ctx, "CALL DOLT_CLONE(?, 'remotes_anon')", remoteURL)
		Expect(err).To(HaveOccurred())

		By("Expecting a clone as the admin user to succeed")
		Expect(sqlClient.Exec(ctx, "CALL DOLT_CLONE('--user', 'root', ?, 'remotes_clone')", remoteURL)).To(Succeed())

		var count int
		Expect(sqlClient.QueryRow(ctx, "SELECT COUNT(*) FROM remotes_clone.t").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(3))
	})
})
