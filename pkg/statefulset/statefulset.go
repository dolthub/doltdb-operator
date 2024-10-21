package statefulset

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ServiceFQDNWithService returns the fully qualified domain name (FQDN) for a service.
func ServiceFQDNWithService(meta metav1.ObjectMeta, service string) string {
	clusterName := os.Getenv("CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", service, meta.Namespace, clusterName)
}

// ServiceFQDN returns the fully qualified domain name (FQDN) for the service associated with the given metadata.
func ServiceFQDN(meta metav1.ObjectMeta) string {
	return ServiceFQDNWithService(meta, meta.Name)
}

// PodName returns the name of a pod given its metadata and index.
func PodName(meta metav1.ObjectMeta, podIndex int) string {
	return fmt.Sprintf("%s-%d", meta.Name, podIndex)
}

// PodFQDNWithService returns the fully qualified domain name (FQDN) for a pod given its metadata, index, and service.
func PodFQDNWithService(meta metav1.ObjectMeta, podIndex int, service string) string {
	return fmt.Sprintf("%s.%s", PodName(meta, podIndex), ServiceFQDNWithService(meta, service))
}

// PodShortFQDNWithService returns the short fully qualified domain name (FQDN) for a pod given its metadata, index, and service.
func PodShortFQDNWithService(meta metav1.ObjectMeta, podIndex int, service string) string {
	return fmt.Sprintf("%s.%s", PodName(meta, podIndex), service)
}

// PodIndex extracts the index from a pod name.
func PodIndex(podName string) (*int, error) {
	parts := strings.Split(podName, "-")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid Pod name: %v", podName)
	}
	index, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return nil, fmt.Errorf("invalid Pod name: %v, error: %v", podName, err)
	}
	return &index, nil
}

// ValidPodName validates if a pod name is valid given the metadata and number of replicas.
func ValidPodName(meta metav1.ObjectMeta, replicas int, podName string) error {
	if replicas < 0 {
		return errors.New("replicas must be positive")
	}

	index, err := PodIndex(podName)
	if err != nil {
		return fmt.Errorf("invalid Pod index: %v", err)
	}
	if *index < 0 || *index >= replicas {
		return fmt.Errorf("index '%d' out of replicas range", *index)
	}

	if !strings.HasPrefix(podName, meta.Name) {
		return fmt.Errorf("invalid Pod name: must start with '%s'", meta.Name)
	}
	return nil
}

// GetStorageSize returns the storage size for a given volume in a StatefulSet.
func GetStorageSize(sts *appsv1.StatefulSet, storageVolume string) *resource.Quantity {
	for _, vctpl := range sts.Spec.VolumeClaimTemplates {
		if vctpl.Name == storageVolume {
			return ptr.To(vctpl.Spec.Resources.Requests[corev1.ResourceStorage])
		}
	}
	return nil
}
