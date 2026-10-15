// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("DoltDB primary termination with automatic failover disabled", func() {
	It("restarts the same pod with its persisted primary role after deletion", func() {
		name := fmt.Sprintf("dolt-no-auto-failover-%d", time.Now().UnixNano())
		doltdbKey := types.NamespacedName{Name: name, Namespace: "default"}
		credentialsKey := types.NamespacedName{Name: name + "-credentials", Namespace: doltdbKey.Namespace}

		credentials := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      credentialsKey.Name,
				Namespace: credentialsKey.Namespace,
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"admin-user":     "root",
				"admin-password": "12345",
			},
		}
		Expect(k8sClient.Create(ctx, credentials)).To(Succeed())

		doltdb := &doltv1alpha.DoltDB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      doltdbKey.Name,
				Namespace: doltdbKey.Namespace,
				Labels:    map[string]string{"k8s.dolthub.com/test": "test"},
				Annotations: map[string]string{
					"k8s.dolthub.com/test": "test",
				},
			},
			Spec: doltv1alpha.DoltDBSpec{
				Image:               "dolthub/dolt",
				EngineVersion:       doltdbEngineVersion,
				Replicas:            3,
				ReplicationStrategy: doltv1alpha.DirectStandby,
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
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				Replication: &doltv1alpha.Replication{
					Enabled: true,
					ReplicationSpec: doltv1alpha.ReplicationSpec{
						Primary: &doltv1alpha.PrimaryReplication{
							PodIndex:          ptr.To(0),
							AutomaticFailover: ptr.To(false),
						},
					},
				},
				Probes: doltv1alpha.Probes{
					LivenessProbe: &corev1.Probe{
						InitialDelaySeconds: 20,
						PeriodSeconds:       5,
						TimeoutSeconds:      5,
					},
					ReadinessProbe: &corev1.Probe{
						InitialDelaySeconds: 15,
						PeriodSeconds:       5,
						TimeoutSeconds:      5,
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, doltdb)).To(Succeed())
		DeferCleanup(func() {
			deleteDoltDB(ctx, doltdbKey, credentialsKey)
		})

		By("waiting for the three-replica cluster to become ready")
		Eventually(func() bool {
			if err := k8sClient.Get(ctx, doltdbKey, doltdb); err != nil {
				return false
			}
			return doltdb.IsReady() && doltdb.Status.CurrentPrimary != nil && doltdb.Status.CurrentPrimaryPodIndex != nil
		}, testHighTimeout, testInterval).Should(BeTrue())

		primaryName := *doltdb.Status.CurrentPrimary
		primaryIndex := *doltdb.Status.CurrentPrimaryPodIndex
		primaryKey := types.NamespacedName{Name: primaryName, Namespace: doltdb.Namespace}
		var originalPrimary corev1.Pod
		Expect(k8sClient.Get(ctx, primaryKey, &originalPrimary)).To(Succeed())
		originalUID := originalPrimary.UID
		primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
		Expect(err).NotTo(HaveOccurred())
		role, _, err := primaryClient.GetRoleAndEpoch(ctx)
		Expect(primaryClient.Close()).To(Succeed())
		Expect(err).NotTo(HaveOccurred())
		Expect(role).To(Equal(dolt.PrimaryRoleValue.String()))

		By("deleting the current primary pod")
		Expect(k8sClient.Delete(ctx, &originalPrimary)).To(Succeed())

		By("verifying the recreated pod retains Dolt's primary role on its PVC")
		var restartedRole string
		Eventually(func() bool {
			var restartedPod corev1.Pod
			if err := k8sClient.Get(ctx, primaryKey, &restartedPod); err != nil {
				return false
			}
			if restartedPod.UID == originalUID || restartedPod.Status.Phase != corev1.PodRunning {
				return false
			}

			primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
			if err != nil {
				return false
			}
			gotRole, _, err := primaryClient.GetRoleAndEpoch(ctx)
			_ = primaryClient.Close()
			if err != nil {
				return false
			}
			restartedRole = gotRole
			return true
		}, testHighTimeout, testInterval).Should(BeTrue())
		Expect(restartedRole).To(Equal(dolt.PrimaryRoleValue.String()))
	})
})
