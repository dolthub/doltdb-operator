package builder

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func doltVolumes(doltcluster *doltv1alpha.DoltCluster) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: doltcluster.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: doltcluster.Name,
				},
			},
		},
	}
}

func doltPodTemplate(doltcluster *doltv1alpha.DoltCluster) corev1.PodTemplateSpec {
	objMetaBuilder := NewMetadataBuilder(client.ObjectKeyFromObject(doltcluster)).
		WithMetadata(doltcluster).
		WithMetadata(doltcluster).Build()

	return corev1.PodTemplateSpec{
		ObjectMeta: objMetaBuilder,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			ServiceAccountName:           doltcluster.Spec.ServiceAccountName,
			Containers:                   doltContainers(doltcluster),
			ImagePullSecrets:             doltcluster.Spec.ImagePullSecrets,
			Volumes:                      doltVolumes(doltcluster),
			SecurityContext:              &doltcluster.Spec.PodSecurityContext,
			Affinity:                     doltcluster.Spec.Affinity,
			NodeSelector:                 doltcluster.Spec.NodeSelector,
			Tolerations:                  doltcluster.Spec.Tolerations,
		},
	}
}
