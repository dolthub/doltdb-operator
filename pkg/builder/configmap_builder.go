package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ConfigMapOpts holds the options for building a ConfigMap.
type ConfigMapOpts struct {
	Metadata *doltv1alpha.DoltCluster
	Key      types.NamespacedName
	Data     map[string]string
}

// BuildConfigMap creates a ConfigMap based on the provided options and sets the owner reference.
// It returns the created ConfigMap or an error if the operation fails.
func (b *Builder) BuildConfigMap(options ConfigMapOpts, owner metav1.Object) (*corev1.ConfigMap, error) {
	objMeta :=
		NewMetadataBuilder(options.Key).
			WithMetadata(options.Metadata).
			Build()
	cm := &corev1.ConfigMap{
		ObjectMeta: objMeta,
		Data:       options.Data,
	}
	if err := controllerutil.SetControllerReference(owner, cm, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to ConfigMap: %v", err)
	}
	return cm, nil
}
