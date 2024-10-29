package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/builder"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var (
	testVeryHighTimeout = 20 * time.Minute
	testHighTimeout     = 10 * time.Minute
	testTimeout         = 5 * time.Minute
	testInterval        = 1 * time.Second

	testDoltKey = types.NamespacedName{
		Name:      "dolt",
		Namespace: fmt.Sprintf("test-namespace-%d", time.Now().Unix()),
	}

	testDoltCredentialsKey = types.NamespacedName{
		Name:      "dolt-credentials",
		Namespace: testDoltKey.Namespace,
	}
)

func testCreateInitialData(ctx context.Context) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testDoltKey.Namespace,
		},
	}
	Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDoltCredentialsKey.Name,
			Namespace: testDoltCredentialsKey.Namespace,
		},
		Type: "Opaque",
		StringData: map[string]string{
			"admin-user":     "root",
			"admin-password": "12345",
		},
	}
	Expect(k8sClient.Create(ctx, &secret)).To(Succeed())

	doltdb := doltv1alpha.DoltCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDoltKey.Name,
			Namespace: testDoltKey.Namespace,
			Labels: map[string]string{
				"k8s.dolthub.com/test": "test",
			},
			Annotations: map[string]string{
				"k8s.dolthub.com/test": "test",
			},
		},
		Spec: doltv1alpha.DoltClusterSpec{
			Image:               "dolthub/dolt",
			EngineVersion:       "1.43.5",
			Replicas:            3,
			ReplicationStrategy: doltv1alpha.DirectStandby,
			Storage: doltv1alpha.Storage{
				Size: ptr.To(resource.MustParse("1Gi")),
			},
			Resources: &v1.ResourceRequirements{
				Requests: v1.ResourceList{
					"cpu":    resource.MustParse("500m"),
					"memory": resource.MustParse("1Gi"),
				},
				Limits: v1.ResourceList{
					"memory": resource.MustParse("1Gi"),
				},
			},
			Replication: &doltv1alpha.Replication{
				Enabled: true,
				ReplicationSpec: doltv1alpha.ReplicationSpec{
					Primary: &doltv1alpha.PrimaryReplication{
						PodIndex:          ptr.To(0),
						AutomaticFailover: ptr.To(true),
					},
				},
			},
		},
	}

	Expect(k8sClient.Create(ctx, &doltdb)).To(Succeed())
	// expectReady(ctx, k8sClient, testDoltKey)
}

func testCleanupInitialData(ctx context.Context) {
	deleteDoltDB(ctx, testDoltKey)
	deleteNamespace(ctx, types.NamespacedName{
		Name:      testDoltKey.Namespace,
		Namespace: testDoltKey.Namespace,
	})
}

func expectReady(ctx context.Context, k8sClient client.Client, key types.NamespacedName) {
	By("Expecting DoltDB to be ready eventually")
	expectFn(ctx, k8sClient, key, func(doltdb *doltv1alpha.DoltCluster) bool {
		return doltdb.IsReady()
	})
}

func expectFn(ctx context.Context, k8sClient client.Client, key types.NamespacedName, fn func(doltdb *doltv1alpha.DoltCluster) bool) {
	var doltdb doltv1alpha.DoltCluster
	Eventually(func(g Gomega) bool {
		g.Expect(k8sClient.Get(ctx, key, &doltdb)).To(Succeed())

		log.FromContext(ctx).Info("Checking DoltDB status", "status", doltdb)

		return fn(&doltdb)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func deleteDoltDB(ctx context.Context, key types.NamespacedName) {
	var doltdb doltv1alpha.DoltCluster
	By("Deleting DoltDB")
	Expect(k8sClient.Get(ctx, key, &doltdb)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &doltdb)).To(Succeed())

	By("Deleting PVCs")
	opts := []client.DeleteAllOfOption{
		client.MatchingLabels(
			builder.NewLabelsBuilder().
				WithDoltSelectorLabels(&doltdb).
				Build(),
		),
		client.InNamespace(doltdb.Namespace),
	}
	Expect(k8sClient.DeleteAllOf(ctx, &corev1.PersistentVolumeClaim{}, opts...)).To(Succeed())
}

func deleteNamespace(ctx context.Context, key types.NamespacedName) {
	var namespace corev1.Namespace
	By("Deleting Namespace")
	Expect(k8sClient.Get(ctx, key, &namespace)).To(Succeed())
	Expect(k8sClient.Delete(ctx, &namespace)).To(Succeed())
}
