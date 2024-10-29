package predicate

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestPredicateChangedWithAnnotations(t *testing.T) {
	s := scheme.Scheme
	s.AddKnownTypes(metav1.SchemeGroupVersion, &metav1.PartialObjectMetadata{})

	annotations := []string{"test-annotation"}
	hasChanged := func(old, new client.Object) bool {
		return old.GetResourceVersion() != new.GetResourceVersion()
	}

	pred := PredicateChangedWithAnnotations(annotations, hasChanged)

	tests := []struct {
		name     string
		oldObj   *metav1.PartialObjectMetadata
		newObj   *metav1.PartialObjectMetadata
		expected bool
	}{
		{
			name: "annotations present and changed",
			oldObj: &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:     map[string]string{"test-annotation": "value"},
					ResourceVersion: "1",
				},
			},
			newObj: &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:     map[string]string{"test-annotation": "value"},
					ResourceVersion: "2",
				},
			},
			expected: true,
		},
		{
			name: "annotations present but not changed",
			oldObj: &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:     map[string]string{"test-annotation": "value"},
					ResourceVersion: "1",
				},
			},
			newObj: &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:     map[string]string{"test-annotation": "value"},
					ResourceVersion: "1",
				},
			},
			expected: false,
		},
		{
			name: "annotations not present",
			oldObj: &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:     map[string]string{},
					ResourceVersion: "1",
				},
			},
			newObj: &metav1.PartialObjectMetadata{
				ObjectMeta: metav1.ObjectMeta{
					Annotations:     map[string]string{},
					ResourceVersion: "2",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := event.UpdateEvent{
				ObjectOld: tt.oldObj,
				ObjectNew: tt.newObj,
			}
			result := pred.Update(e)
			if result != tt.expected {
				t.Errorf("expected %v, but got %v", tt.expected, result)
			}
		})
	}
}
