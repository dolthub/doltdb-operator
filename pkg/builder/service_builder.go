package builder

import (
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DoltHeadlessService = "dolt-internal"
	DoltPrimaryService  = "dolt"
	DoltReaderService   = "dolt-ro"
)

// doltServicePorts returns the service ports for the Dolt cluster.
func doltServicePorts(doltcluster *doltv1alpha.DoltCluster) []v1.ServicePort {
	return []v1.ServicePort{
		{
			Port: DoltMySQLPort,
			Name: doltcluster.Name,
		},
	}
}

// BuildDoltInternalService creates a headless service for the Dolt cluster.
func (b *Builder) BuildDoltInternalService(doltcluster *doltv1alpha.DoltCluster) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltcluster.InternalServiceKey()).
		WithMetadata(doltcluster).
		WithMetadata(doltcluster).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltcluster).Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:     doltServicePorts(doltcluster),
			ClusterIP: "None",
			Selector:  labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltcluster, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}

// BuildDoltPrimaryService creates a primary service for the Dolt cluster.
func (b *Builder) BuildDoltPrimaryService(doltcluster *doltv1alpha.DoltCluster) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltcluster.PrimaryServiceKey()).
		WithMetadata(doltcluster).
		WithMetadata(doltcluster).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltcluster).WithPodPrimaryRole().Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:    doltServicePorts(doltcluster),
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltcluster, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}

// BuildDoltReaderService creates a reader service for the Dolt cluster.
func (b *Builder) BuildDoltReaderService(doltcluster *doltv1alpha.DoltCluster) (*v1.Service, error) {
	objMeta := NewMetadataBuilder(doltcluster.ReaderServiceKey()).
		WithMetadata(doltcluster).
		WithMetadata(doltcluster).Build()

	labels := NewLabelsBuilder().WithDoltSelectorLabels(doltcluster).WithPodStandbyRole().Build()

	svc := &v1.Service{
		ObjectMeta: objMeta,
		Spec: v1.ServiceSpec{
			Ports:    doltServicePorts(doltcluster),
			Type:     v1.ServiceTypeClusterIP,
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(doltcluster, svc, b.scheme); err != nil {
		return nil, fmt.Errorf("error setting controller reference to Service: %v", err)
	}

	return svc, nil
}
