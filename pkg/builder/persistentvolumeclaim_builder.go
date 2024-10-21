package builder

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	DoltStorageVolumeRole = "storage"
)

// BuildStoragePVC constructs a PersistentVolumeClaim for the given DoltCluster.
func (b *Builder) BuildStoragePVC(key types.NamespacedName, doltcluster *doltv1alpha.DoltCluster) *corev1.PersistentVolumeClaim {
	labels := NewLabelsBuilder().
		WithDoltSelectorLabels(doltcluster).
		WithPVCRole(DoltStorageVolumeRole).
		Build()

	objMeta :=
		NewMetadataBuilder(key).
			WithMetadata(doltcluster).
			WithLabels(labels).
			Build()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: objMeta,
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: doltcluster.Spec.Volume.Size,
				},
			},
		},
	}

	if doltcluster.Spec.Volume.StorageClass != nil {
		pvc.Spec.StorageClassName = doltcluster.Spec.Volume.StorageClass
	}

	return pvc
}
