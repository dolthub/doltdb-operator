package builder

import (
	appsv1 "k8s.io/api/apps/v1"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// BuildDoltStatefulSet constructs a StatefulSet for a DoltCluster based on the provided NamespacedName and DoltCluster object.
// It sets up the metadata, labels, volume claim templates, and pod template for the StatefulSet.
func (b *Builder) BuildDoltStatefulSet(key types.NamespacedName, doltcluster *doltv1alpha.DoltCluster) (*appsv1.StatefulSet, error) {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltcluster).
		WithVersion(doltcluster.Spec.EngineVersion).
		Build()

	objMeta := NewMetadataBuilder(key).
		WithMetadata(doltcluster).
		WithLabels(labels).
		Build()

	storagePVC := b.BuildStoragePVC(key, doltcluster)
	volumeClaimTemplates := []corev1.PersistentVolumeClaim{
		*storagePVC,
	}
	podTemplate := doltPodTemplate(doltcluster)

	matchLabels := NewLabelsBuilder().WithDoltSelectorLabels(doltcluster).Build()

	return &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			ServiceName:          doltcluster.InternalServiceKey().Name,
			Replicas:             doltcluster.Spec.Replicas,
			Template:             podTemplate,
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}, nil
}
