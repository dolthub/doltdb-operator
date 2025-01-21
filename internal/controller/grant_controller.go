/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/watch"
	"k8s.io/apimachinery/pkg/util/wait"
)

// GrantReconciler reconciles a Grant object
type GrantReconciler struct {
	client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
	SqlOpts        []database.SqlOpt
}

func NewGrantReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *conditions.Ready,
	sqlOpts ...database.SqlOpt) *GrantReconciler {
	return &GrantReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		SqlOpts:        sqlOpts,
	}
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=grants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=grants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=grants/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GrantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.V(1).Info("Reconciling Grant", "grant", req.NamespacedName)

	var grant doltv1alpha.Grant
	if err := r.Get(ctx, req.NamespacedName, &grant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	grantReconciler := newWrappedGrantReconciler(r.Client, *r.RefResolver, &grant)
	grantFinalizer := newWrappedGrantFinalizer(r.Client, &grant)
	finalizerCtrl := database.NewSqlFinalizer(r.Client, grantFinalizer, r.SqlOpts...)
	grantCtrl := database.NewSqlReconciler(r.Client, r.ConditionReady, grantReconciler, finalizerCtrl, r.SqlOpts...)

	result, err := grantCtrl.Reconcile(ctx, &grant)
	if err != nil {
		return result, fmt.Errorf("error reconciling GrantController: %v", err)
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GrantReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.Grant{})

	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, r.Client)
	if err := watcherIndexer.Watch(
		ctx,
		&doltv1alpha.User{},
		&doltv1alpha.Grant{},
		&doltv1alpha.GrantList{},
		doltv1alpha.GrantUsernameFieldPath,
		ctrlbuilder.WithPredicates(predicate.Funcs{
			CreateFunc: func(ce event.CreateEvent) bool {
				return true
			},
		}),
	); err != nil {
		return fmt.Errorf("error watching grants: %v", err)
	}

	return builder.Complete(r)
}

type wrappedGrantReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	grant       *doltv1alpha.Grant
}

func newWrappedGrantReconciler(client client.Client, refResolver refresolver.RefResolver,
	grant *doltv1alpha.Grant) database.WrappedReconciler {
	return &wrappedGrantReconciler{
		Client:      client,
		refResolver: &refResolver,
		grant:       grant,
	}
}

func (wr *wrappedGrantReconciler) Reconcile(ctx context.Context, doltdbClient *sql.Client) error {
	var opts []sql.GrantOption
	if wr.grant.Spec.GrantOption {
		opts = append(opts, sql.WithGrantOption())
	}
	if err := doltdbClient.Grant(
		ctx,
		wr.grant.Spec.Privileges,
		wr.grant.Spec.Database,
		wr.grant.Spec.Table,
		wr.grant.AccountName(),
		opts...,
	); err != nil {
		return fmt.Errorf("error granting privileges in DoltDB: %v", err)
	}
	return nil
}

func (wr *wrappedGrantReconciler) PatchStatus(ctx context.Context, patcher conditions.Patcher) error {
	patch := client.MergeFrom(wr.grant.DeepCopy())
	patcher(&wr.grant.Status)

	if err := wr.Client.Status().Patch(ctx, wr.grant, patch); err != nil {
		return fmt.Errorf("error patching Grant status: %v", err)
	}
	return nil
}

type wrappedGrantFinalizer struct {
	client.Client
	grant *doltv1alpha.Grant
}

func newWrappedGrantFinalizer(client client.Client, grant *doltv1alpha.Grant) database.WrappedFinalizer {
	return &wrappedGrantFinalizer{
		Client: client,
		grant:  grant,
	}
}

func (wf *wrappedGrantFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.grant, func(gmd *doltv1alpha.Grant) {
		controllerutil.AddFinalizer(wf.grant, dolt.GrantFinalizerName)
	})
}

func (wf *wrappedGrantFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.grant, func(gmd *doltv1alpha.Grant) {
		controllerutil.RemoveFinalizer(wf.grant, dolt.GrantFinalizerName)
	})
}

func (wf *wrappedGrantFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wf.grant, dolt.GrantFinalizerName)
}

func (wf *wrappedGrantFinalizer) Reconcile(ctx context.Context, doltdbClient *sql.Client) error {
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
		var user doltv1alpha.User
		if err := wf.Get(ctx, userKey(wf.grant), &user); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
	// User does not exist
	if err == nil {
		return nil
	}
	if !wait.Interrupted(err) {
		return fmt.Errorf("error checking if user exists in DoltDB: %v", err)
	}

	var opts []sql.GrantOption
	if wf.grant.Spec.GrantOption {
		opts = append(opts, sql.WithGrantOption())
	}
	if err := doltdbClient.Revoke(
		ctx,
		wf.grant.Spec.Privileges,
		wf.grant.Spec.Database,
		wf.grant.Spec.Table,
		wf.grant.AccountName(),
		opts...,
	); err != nil {
		return fmt.Errorf("error revoking grant in DoltDB: %v", err)
	}
	return nil
}

func (wf *wrappedGrantFinalizer) patch(ctx context.Context, grant *doltv1alpha.Grant,
	patchFn func(*doltv1alpha.Grant)) error {
	patch := client.MergeFrom(grant.DeepCopy())
	patchFn(grant)

	if err := wf.Client.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error patching Grant: %v", err)
	}
	return nil
}

func userKey(grant *doltv1alpha.Grant) types.NamespacedName {
	return types.NamespacedName{
		Name:      grant.Spec.Username,
		Namespace: grant.Namespace,
	}
}
