// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"fmt"
	"sort"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	podpkg "github.com/electronicarts/doltdb-operator/pkg/pod"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
		Expect(err).NotTo(HaveOccurred())
		role, epoch, err := primaryClient.GetRoleAndEpoch(ctx)
		Expect(primaryClient.Close()).To(Succeed())
		Expect(err).NotTo(HaveOccurred())
		Expect(role).To(Equal(dolt.PrimaryRoleValue.String()))

		By("deleting the current primary pod")
		Expect(k8sClient.Delete(ctx, &originalPrimary)).To(Succeed())
		waitForPodRecreated(primaryKey, originalPrimary.UID)

		By("verifying the recreated pod retains Dolt's primary role and epoch on its PVC")
		Eventually(func() string {
			return manualFailoverRoleMismatch(doltdb, primaryIndex, epoch)
		}, testHighTimeout, testInterval).Should(BeEmpty())
	})

	It("reasserts the last primary after a legacy preStop demotion", func() {
		name := fmt.Sprintf("dolt-legacy-prestop-reassert-%d", time.Now().UnixNano())
		doltdbKey := types.NamespacedName{Name: name, Namespace: "default"}
		doltdb := createReplicationCluster(name, "", false)
		waitForManualFailoverCluster(doltdbKey, doltdb)

		primaryIndex := *doltdb.Status.CurrentPrimaryPodIndex
		primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
		Expect(err).NotTo(HaveOccurred())
		role, epoch, err := primaryClient.GetRoleAndEpoch(ctx)
		Expect(primaryClient.Close()).To(Succeed())
		Expect(err).NotTo(HaveOccurred())
		Expect(role).To(Equal(dolt.PrimaryRoleValue.String()))
		seededDatabase := expectPrimaryWritable(doltdb, primaryIndex)
		waitForManualFailoverReplicasCaughtUp(doltdb, primaryIndex, seededDatabase)

		var primaryPod corev1.Pod
		primaryKey := types.NamespacedName{Name: *doltdb.Status.CurrentPrimary, Namespace: doltdb.Namespace}
		Expect(k8sClient.Get(ctx, primaryKey, &primaryPod)).To(Succeed())

		By("running the legacy preStop demotion SQL, then deleting the pod")
		// Same procedure call as the legacy hook. The operator may reassert before the
		// deletion lands, so the assertions below only accept the recreated pod.
		primaryClient, err = sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
		Expect(err).NotTo(HaveOccurred())
		err = primaryClient.Exec(ctx, fmt.Sprintf("CALL DOLT_CLUSTER_TRANSITION_TO_STANDBY('%d', '1')", epoch+1))
		Expect(primaryClient.Close()).To(Succeed())
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, &primaryPod)).To(Succeed())
		waitForPodRecreated(primaryKey, primaryPod.UID)

		By("reasserting the recorded primary at the next epoch and accepting writes")
		Eventually(func() string {
			return manualFailoverRoleMismatch(doltdb, primaryIndex, epoch+2)
		}, testHighTimeout, testInterval).Should(BeEmpty())
		Eventually(func() bool {
			var latest doltv1alpha.DoltDB
			if err := k8sClient.Get(ctx, doltdbKey, &latest); err != nil {
				return false
			}
			return ptr.Deref(latest.Status.CurrentPrimaryPodIndex, -1) == primaryIndex &&
				ptr.Deref(latest.Status.ReplicationEpoch, -1) == epoch+2 &&
				meta.IsStatusConditionTrue(latest.Status.Conditions, doltv1alpha.ConditionTypePrimaryAvailable)
		}, testHighTimeout, testInterval).Should(BeTrue())
		expectPrimaryWritable(doltdb, primaryIndex)

		By("replicating a fresh write from the reasserted primary to every standby")
		// The seeded database already replicated once before the demotion, so a second row
		// isolates one question: did replication resume at the reasserted epoch.
		insertWritableRow(doltdb, primaryIndex, seededDatabase, 2)
		Eventually(func() string {
			return standbyRowCountMismatch(doltdb, primaryIndex, seededDatabase, 2)
		}, testHighTimeout, testInterval).Should(BeEmpty())
	})

	It("rolls out RollingUpdate changes while keeping the pinned primary when automatic failover is disabled", func() {
		name := fmt.Sprintf("dolt-manual-failover-rollout-%d", time.Now().UnixNano())
		doltdbKey := types.NamespacedName{Name: name, Namespace: "default"}
		// Bootstrap always starts pod 0 as primary, so pin index 1 through the supported manual switchover path.
		doltdb := createReplicationCluster(name, doltv1alpha.RollingUpdateUpdateType, false)
		waitForManualFailoverCluster(doltdbKey, doltdb)
		switchPinnedPrimary(doltdbKey, doltdb, 1)

		By("waiting for Dolt roles to match the pinned primary before rolling out")
		Eventually(func() string {
			return manualFailoverRoleMismatch(doltdb, 1, -1)
		}, testHighTimeout, testInterval).Should(BeEmpty())
		primaryEpoch := podEpoch(doltdb, 1)
		seededDatabase := expectPrimaryWritable(doltdb, 1)
		waitForManualFailoverReplicasCaughtUp(doltdb, 1, seededDatabase)

		statefulSet := waitForStatefulSetObserved(doltdbKey)
		Expect(statefulSet.Spec.UpdateStrategy.Type).To(Equal(appsv1.OnDeleteStatefulSetStrategyType))

		By("changing a pod-template annotation to trigger operator-managed rollout")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, doltdbKey, doltdb); err != nil {
				return err
			}
			doltdb.Spec.PodAnnotations = map[string]string{"rollout-trigger": "changed"}
			return k8sClient.Update(ctx, doltdb)
		}, testHighTimeout, testInterval).Should(Succeed())

		pods := waitForOperatorRollout(doltdbKey, doltdb, statefulSet.Status.UpdateRevision)
		for i := range pods {
			Expect(pods[i].Annotations).To(HaveKeyWithValue("rollout-trigger", "changed"))
		}
		expectStandbysRestartedBeforePrimary(pods, 1)

		By("keeping pod index 1 primary at the same epoch without a handoff")
		waitForPinnedPrimaryStatus(doltdbKey, 1)
		Eventually(func() string {
			return manualFailoverRoleMismatch(doltdb, 1, primaryEpoch)
		}, testHighTimeout, testInterval).Should(BeEmpty())
		expectPrimaryWritable(doltdb, 1)
	})

	It("recovers the pinned primary when an upgrade rolls out pods that still carry the legacy preStop hook", func() {
		name := fmt.Sprintf("dolt-legacy-hook-upgrade-%d", time.Now().UnixNano())
		doltdbKey := types.NamespacedName{Name: name, Namespace: "default"}
		// automaticFailover: true renders the same preStop hook every replicated pod received before the
		// hook was gated, so these pods match those created by the previous operator release.
		doltdb := createReplicationCluster(name, doltv1alpha.RollingUpdateUpdateType, true)
		waitForManualFailoverCluster(doltdbKey, doltdb)
		// Native RollingUpdate replaces the highest ordinal first; a primary at ordinal 1 is replaced
		// before standby 0, which is the failing order from the upgrade report.
		switchPinnedPrimary(doltdbKey, doltdb, 1)

		Eventually(func() string {
			return manualFailoverRoleMismatch(doltdb, 1, -1)
		}, testHighTimeout, testInterval).Should(BeEmpty())
		primaryEpoch := podEpoch(doltdb, 1)
		seededDatabase := expectPrimaryWritable(doltdb, 1)
		waitForManualFailoverReplicasCaughtUp(doltdb, 1, seededDatabase)

		statefulSet := waitForStatefulSetObserved(doltdbKey)
		Expect(statefulSet.Spec.UpdateStrategy.Type).To(Equal(appsv1.RollingUpdateStatefulSetStrategyType))
		for i := 0; i < int(doltdb.Spec.Replicas); i++ {
			var pod corev1.Pod
			podKey := types.NamespacedName{Name: fmt.Sprintf("%s-%d", doltdb.Name, i), Namespace: doltdb.Namespace}
			Expect(k8sClient.Get(ctx, podKey, &pod)).To(Succeed())
			Expect(hasPreStopHook(&pod)).To(BeTrue(), "pod %s should start with the legacy preStop hook", pod.Name)
		}

		By("disabling automatic failover, which removes the hook from the pod template")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, doltdbKey, doltdb); err != nil {
				return err
			}
			doltdb.Spec.Replication.Primary.AutomaticFailover = ptr.To(false)
			return k8sClient.Update(ctx, doltdb)
		}, testHighTimeout, testInterval).Should(Succeed())

		pods := waitForOperatorRollout(doltdbKey, doltdb, statefulSet.Status.UpdateRevision)
		Expect(k8sClient.Get(ctx, doltdbKey, &statefulSet)).To(Succeed())
		Expect(statefulSet.Spec.UpdateStrategy.Type).To(Equal(appsv1.OnDeleteStatefulSetStrategyType))
		for i := range pods {
			Expect(hasPreStopHook(&pods[i])).To(BeFalse(), "pod %s should be recreated without the preStop hook", pods[i].Name)
		}
		expectStandbysRestartedBeforePrimary(pods, 1)

		By("recovering from the legacy hook demotion with pod index 1 as the only writable primary")
		waitForPinnedPrimaryStatus(doltdbKey, 1)
		// The seeded data is fully replicated and the operator waits for restarted standbys to catch up,
		// so the old hook's transition succeeds (+1 epoch) and the reassertion promotes it again (+1 epoch).
		Eventually(func() string {
			return manualFailoverRoleMismatch(doltdb, 1, primaryEpoch+2)
		}, testHighTimeout, testInterval).Should(BeEmpty())
		Eventually(func() bool {
			return hasDoltDBEvent(doltdbKey, doltv1alpha.ReasonPrimaryReasserted)
		}, testHighTimeout, testInterval).Should(BeTrue())
		expectWritableRowCount(doltdb, 1, seededDatabase, 1)
		expectPrimaryWritable(doltdb, 1)

		By("replicating a fresh write from the reasserted primary to every standby")
		// The seeded database already replicated once before the rollout, so a second row
		// isolates one question: did replication resume at the reasserted epoch.
		insertWritableRow(doltdb, 1, seededDatabase, 2)
		Eventually(func() string {
			return standbyRowCountMismatch(doltdb, 1, seededDatabase, 2)
		}, testHighTimeout, testInterval).Should(BeEmpty())
	})
})

func createReplicationCluster(name string, updateStrategy doltv1alpha.UpdateType, automaticFailover bool) *doltv1alpha.DoltDB {
	doltdbKey := types.NamespacedName{Name: name, Namespace: "default"}
	credentialsKey := types.NamespacedName{Name: name + "-credentials", Namespace: doltdbKey.Namespace}
	credentials := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: credentialsKey.Name, Namespace: credentialsKey.Namespace},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"admin-user": "root", "admin-password": "12345"},
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
			UpdateStrategy:      updateStrategy,
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
				Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
			},
			Replication: &doltv1alpha.Replication{
				Enabled: true,
				ReplicationSpec: doltv1alpha.ReplicationSpec{
					Primary: &doltv1alpha.PrimaryReplication{
						PodIndex:          ptr.To(0),
						AutomaticFailover: ptr.To(automaticFailover),
					},
				},
			},
			Probes: doltv1alpha.Probes{
				LivenessProbe:  &corev1.Probe{InitialDelaySeconds: 20, PeriodSeconds: 5, TimeoutSeconds: 5},
				ReadinessProbe: &corev1.Probe{InitialDelaySeconds: 15, PeriodSeconds: 5, TimeoutSeconds: 5},
			},
		},
	}
	Expect(k8sClient.Create(ctx, doltdb)).To(Succeed())
	DeferCleanup(func() {
		deleteDoltDB(ctx, doltdbKey, credentialsKey)
	})
	return doltdb
}

func waitForManualFailoverCluster(doltdbKey types.NamespacedName, doltdb *doltv1alpha.DoltDB) {
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, doltdbKey, doltdb); err != nil {
			return false
		}
		return doltdb.IsReady() && doltdb.Status.CurrentPrimary != nil && doltdb.Status.CurrentPrimaryPodIndex != nil
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func waitForPodRecreated(podKey types.NamespacedName, previousUID types.UID) {
	Eventually(func() bool {
		var pod corev1.Pod
		if err := k8sClient.Get(ctx, podKey, &pod); err != nil {
			return false
		}
		return pod.UID != previousUID && pod.DeletionTimestamp == nil && podpkg.PodReady(&pod)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func switchPinnedPrimary(doltdbKey types.NamespacedName, doltdb *doltv1alpha.DoltDB, index int) {
	By(fmt.Sprintf("switching the pinned primary to pod index %d", index))
	Eventually(func() error {
		if err := k8sClient.Get(ctx, doltdbKey, doltdb); err != nil {
			return err
		}
		doltdb.Spec.Replication.Primary.PodIndex = ptr.To(index)
		return k8sClient.Update(ctx, doltdb)
	}, testHighTimeout, testInterval).Should(Succeed())
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, doltdbKey, doltdb); err != nil {
			return false
		}
		return doltdb.IsReady() && !doltdb.IsSwitchingPrimary() && ptr.Deref(doltdb.Status.CurrentPrimaryPodIndex, -1) == index
	}, testHighTimeout, testInterval).Should(BeTrue())
}

func podEpoch(doltdb *doltv1alpha.DoltDB, index int) int {
	podClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, index)
	Expect(err).NotTo(HaveOccurred())
	_, epoch, err := podClient.GetRoleAndEpoch(ctx)
	Expect(podClient.Close()).To(Succeed())
	Expect(err).NotTo(HaveOccurred())
	return epoch
}

func hasPreStopHook(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Lifecycle != nil && container.Lifecycle.PreStop != nil {
			return true
		}
	}
	return false
}

func hasDoltDBEvent(doltdbKey types.NamespacedName, reason string) bool {
	var events corev1.EventList
	if err := k8sClient.List(ctx, &events, client.InNamespace(doltdbKey.Namespace)); err != nil {
		return false
	}
	for _, event := range events.Items {
		if event.InvolvedObject.Kind == "DoltDB" && event.InvolvedObject.Name == doltdbKey.Name && event.Reason == reason {
			return true
		}
	}
	return false
}

func waitForStatefulSetObserved(doltdbKey types.NamespacedName) appsv1.StatefulSet {
	var statefulSet appsv1.StatefulSet
	Eventually(func() bool {
		if err := k8sClient.Get(ctx, doltdbKey, &statefulSet); err != nil {
			return false
		}
		return statefulSet.Status.ObservedGeneration == statefulSet.Generation && statefulSet.Status.UpdateRevision != ""
	}, testHighTimeout, testInterval).Should(BeTrue())
	return statefulSet
}

// waitForOperatorRollout waits until the StatefulSet observes a revision other than oldRevision
// and every pod is a ready, non-terminating pod from that revision. It returns those pods by ordinal.
func waitForOperatorRollout(doltdbKey types.NamespacedName, doltdb *doltv1alpha.DoltDB, oldRevision string) []corev1.Pod {
	By("waiting for every pod to be recreated from the new StatefulSet revision")
	pods := make([]corev1.Pod, doltdb.Spec.Replicas)
	Eventually(func() string {
		var statefulSet appsv1.StatefulSet
		if err := k8sClient.Get(ctx, doltdbKey, &statefulSet); err != nil {
			return fmt.Sprintf("getting StatefulSet: %v", err)
		}
		targetRevision := statefulSet.Status.UpdateRevision
		if statefulSet.Status.ObservedGeneration != statefulSet.Generation || targetRevision == "" || targetRevision == oldRevision {
			return fmt.Sprintf("StatefulSet generation=%d observedGeneration=%d updateRevision=%q oldRevision=%q",
				statefulSet.Generation, statefulSet.Status.ObservedGeneration, targetRevision, oldRevision)
		}
		var pending []string
		for i := range pods {
			podKey := types.NamespacedName{Name: fmt.Sprintf("%s-%d", doltdb.Name, i), Namespace: doltdb.Namespace}
			if err := k8sClient.Get(ctx, podKey, &pods[i]); err != nil {
				pending = append(pending, fmt.Sprintf("%s: get failed: %v", podKey.Name, err))
				continue
			}
			pod := &pods[i]
			if pod.Labels["controller-revision-hash"] != targetRevision || pod.DeletionTimestamp != nil || !podpkg.PodReady(pod) {
				pending = append(pending, fmt.Sprintf("%s: revision=%q phase=%s ready=%t terminating=%t",
					pod.Name, pod.Labels["controller-revision-hash"], pod.Status.Phase, podpkg.PodReady(pod), pod.DeletionTimestamp != nil))
			}
		}
		if len(pending) > 0 {
			return fmt.Sprintf("targetRevision=%q currentRevision=%q pendingPods=%v", targetRevision, statefulSet.Status.CurrentRevision, pending)
		}
		return ""
	}, testHighTimeout, testInterval).Should(BeEmpty())
	return pods
}

// expectStandbysRestartedBeforePrimary relies on the operator waiting for restarted standbys to become
// ready, which spans the readiness delay and keeps creation timestamps at least seconds apart.
func expectStandbysRestartedBeforePrimary(pods []corev1.Pod, primaryIndex int) {
	By("checking standbys were recreated before the pinned primary")
	for i := range pods {
		if i == primaryIndex {
			continue
		}
		Expect(pods[primaryIndex].CreationTimestamp.After(pods[i].CreationTimestamp.Time)).To(BeTrue(),
			"primary %s created at %s, standby %s created at %s",
			pods[primaryIndex].Name, pods[primaryIndex].CreationTimestamp, pods[i].Name, pods[i].CreationTimestamp)
	}
}

func waitForPinnedPrimaryStatus(doltdbKey types.NamespacedName, primaryIndex int) {
	Eventually(func() bool {
		var latest doltv1alpha.DoltDB
		if err := k8sClient.Get(ctx, doltdbKey, &latest); err != nil {
			return false
		}
		return latest.IsReady() && ptr.Deref(latest.Status.CurrentPrimaryPodIndex, -1) == primaryIndex &&
			meta.IsStatusConditionTrue(latest.Status.Conditions, doltv1alpha.ConditionTypeUpdated) &&
			meta.IsStatusConditionTrue(latest.Status.Conditions, doltv1alpha.ConditionTypePrimaryAvailable)
	}, testHighTimeout, testInterval).Should(BeTrue())
}

// manualFailoverRoleMismatch returns "" when Dolt reports exactly one primary at
// primaryIndex and every other pod is a standby at an epoch no higher than the primary's.
// A negative primaryEpoch accepts any primary epoch.
func manualFailoverRoleMismatch(doltdb *doltv1alpha.DoltDB, primaryIndex, primaryEpoch int) string {
	roles := make([]string, doltdb.Spec.Replicas)
	epochs := make([]int, doltdb.Spec.Replicas)
	for i := range roles {
		podClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, i)
		if err != nil {
			return fmt.Sprintf("connecting to pod %d: %v", i, err)
		}
		roles[i], epochs[i], err = podClient.GetRoleAndEpoch(ctx)
		_ = podClient.Close()
		if err != nil {
			return fmt.Sprintf("reading role of pod %d: %v", i, err)
		}
	}

	if roles[primaryIndex] != dolt.PrimaryRoleValue.String() ||
		(primaryEpoch >= 0 && epochs[primaryIndex] != primaryEpoch) {
		return fmt.Sprintf("pod %d is %q at epoch %d, want primary at epoch %d (roles=%v epochs=%v)",
			primaryIndex, roles[primaryIndex], epochs[primaryIndex], primaryEpoch, roles, epochs)
	}
	for i := range roles {
		if i != primaryIndex && (roles[i] != dolt.StandbyRoleValue.String() || epochs[i] > epochs[primaryIndex]) {
			return fmt.Sprintf("pod %d is %q at epoch %d, want standby at or below primary epoch %d (roles=%v epochs=%v)",
				i, roles[i], epochs[i], epochs[primaryIndex], roles, epochs)
		}
	}
	return ""
}

// waitForManualFailoverReplicasCaughtUp waits until a standby is caught up on every database the
// primary reports, including requiredDatabase. Naming the database is what makes the wait sound:
// Dolt's own TRANSITION_TO_STANDBY enumerates the real database set, but a just-created database
// registers no dolt_cluster_status rows for a short window, so a check that infers its database set
// from those rows alone can report "caught up" while the newest database has reached no standby.
func waitForManualFailoverReplicasCaughtUp(doltdb *doltv1alpha.DoltDB, primaryIndex int, requiredDatabase string) {
	Eventually(func() string {
		primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
		if err != nil {
			return fmt.Sprintf("connecting to primary: %v", err)
		}
		statuses, err := primaryClient.GetClusterStatus(ctx)
		closeErr := primaryClient.Close()
		if err != nil {
			return fmt.Sprintf("reading cluster status: %v", err)
		}
		if closeErr != nil {
			return fmt.Sprintf("closing primary client: %v", closeErr)
		}
		if !hasStandbyCaughtUpAcrossDatabases(statuses, requiredDatabase) {
			return fmt.Sprintf("no standby is caught up across all databases (require %q, saw %v): %+v",
				requiredDatabase, statusDatabases(statuses), statuses)
		}
		return ""
	}, testHighTimeout, testInterval).Should(BeEmpty())
}

// statusDatabases lists the databases present in the status rows, for failure diagnostics.
func statusDatabases(statuses []dolt.DoltStatus) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if _, ok := seen[status.Database]; ok {
			continue
		}
		seen[status.Database] = struct{}{}
		names = append(names, status.Database)
	}
	sort.Strings(names)
	return names
}

// hasStandbyCaughtUpAcrossDatabases mirrors Dolt's caught-up check: lag is 0 only when the last
// pushed head equals the next head, and NULL until the first push. Dolt reports standby_remote as
// the configured remote name, so each standby has one stable key across databases.
// requiredDatabase must appear in the rows, otherwise a database that has not yet registered
// replication status would be silently excluded from the "all databases" comparison below.
func hasStandbyCaughtUpAcrossDatabases(statuses []dolt.DoltStatus, requiredDatabase string) bool {
	databases := make(map[string]struct{})
	remoteDatabases := make(map[string]map[string]bool)
	for _, status := range statuses {
		databases[status.Database] = struct{}{}
		if status.Remote == "" {
			continue
		}

		caughtUp := status.ReplicationLag.Valid && status.ReplicationLag.Int64 == 0 &&
			(!status.CurrentError.Valid || status.CurrentError.String == "")
		remote := status.Remote
		if _, ok := remoteDatabases[remote]; !ok {
			remoteDatabases[remote] = make(map[string]bool)
		}
		previous, exists := remoteDatabases[remote][status.Database]
		remoteDatabases[remote][status.Database] = caughtUp && (!exists || previous)
	}

	if _, ok := databases[requiredDatabase]; !ok {
		return false
	}

	for _, remoteStatus := range remoteDatabases {
		if len(remoteStatus) != len(databases) {
			continue
		}
		allCaughtUp := true
		for _, caughtUp := range remoteStatus {
			allCaughtUp = allCaughtUp && caughtUp
		}
		if allCaughtUp {
			return true
		}
	}
	return false
}

// expectPrimaryWritable creates a committed database with one row and returns its name.
func expectPrimaryWritable(doltdb *doltv1alpha.DoltDB, primaryIndex int) string {
	primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex)
	Expect(err).NotTo(HaveOccurred())
	databaseName := fmt.Sprintf("manual_failover_%d", time.Now().UnixNano())
	Expect(primaryClient.Exec(ctx, "CREATE DATABASE `"+databaseName+"`")).To(Succeed())
	Expect(primaryClient.Close()).To(Succeed())

	primaryClient, err = sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex, sql.WithDatabase(databaseName))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		_ = primaryClient.Close()
	})
	Expect(primaryClient.Exec(ctx, "CREATE TABLE writable (id INT PRIMARY KEY)")).To(Succeed())
	Expect(primaryClient.Exec(ctx, "INSERT INTO writable VALUES (1)")).To(Succeed())
	Expect(primaryClient.Exec(ctx, "CALL DOLT_COMMIT('-Am', 'seed manual failover test replication')")).To(Succeed())
	return databaseName
}

func expectWritableRowCount(doltdb *doltv1alpha.DoltDB, podIndex int, databaseName string, want int) {
	podClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, podIndex, sql.WithDatabase(databaseName))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		_ = podClient.Close()
	})
	var got int
	Expect(podClient.QueryRow(ctx, "SELECT COUNT(*) FROM writable").Scan(&got)).To(Succeed())
	Expect(got).To(Equal(want))
}

// insertWritableRow commits one more row into an already-replicated database on the primary.
// Dolt cluster replication ships commits, so the write must be committed to propagate.
func insertWritableRow(doltdb *doltv1alpha.DoltDB, primaryIndex int, databaseName string, id int) {
	primaryClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, primaryIndex, sql.WithDatabase(databaseName))
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() {
		_ = primaryClient.Close()
	})
	Expect(primaryClient.Exec(ctx, "INSERT INTO writable VALUES (?)", id)).To(Succeed())
	Expect(primaryClient.Exec(ctx, "CALL DOLT_COMMIT('-Am', 'post-reassert replication probe')")).To(Succeed())
}

// standbyRowCountMismatch reports why a standby has not yet received the primary's latest write,
// or "" once every standby has it. Returns a reason rather than asserting so that callers can poll
// it with Eventually: replication is asynchronous, so a one-shot read would flake.
func standbyRowCountMismatch(doltdb *doltv1alpha.DoltDB, primaryIndex int, databaseName string, want int) string {
	for i := 0; i < int(doltdb.Spec.Replicas); i++ {
		if i == primaryIndex {
			continue
		}
		podClient, err := sql.NewInternalClientWithPodIndex(ctx, doltdb, refResolver, i, sql.WithDatabase(databaseName))
		if err != nil {
			return fmt.Sprintf("connecting to standby %d: %v", i, err)
		}
		var got int
		err = podClient.QueryRow(ctx, "SELECT COUNT(*) FROM writable").Scan(&got)
		closeErr := podClient.Close()
		if err != nil {
			return fmt.Sprintf("reading row count on standby %d: %v", i, err)
		}
		if closeErr != nil {
			return fmt.Sprintf("closing standby %d client: %v", i, closeErr)
		}
		if got != want {
			return fmt.Sprintf("standby %d has %d rows, want %d", i, got, want)
		}
	}
	return ""
}
