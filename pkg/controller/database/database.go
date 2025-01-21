package database

import (
	"context"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Resource interface {
	v1.Object
	DoltDBRef() *doltv1alpha.DoltDBRef
	IsBeingDeleted() bool
	RequeueInterval() *metav1.Duration
	RetryInterval() *metav1.Duration
	CleanupPolicy() *doltv1alpha.CleanupPolicy
}

type Reconciler interface {
	Reconcile(ctx context.Context, resource Resource) (ctrl.Result, error)
}

type WrappedReconciler interface {
	Reconcile(context.Context, *sql.Client) error
	PatchStatus(context.Context, conditions.Patcher) error
}

type Finalizer interface {
	AddFinalizer(context.Context) error
	Finalize(context.Context, Resource) (ctrl.Result, error)
}

type WrappedFinalizer interface {
	AddFinalizer(context.Context) error
	RemoveFinalizer(context.Context) error
	ContainsFinalizer() bool
	Reconcile(context.Context, *sql.Client) error
}
