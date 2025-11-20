// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	"github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/predicate"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/electronicarts/doltdb-operator/pkg/watch"
	corev1 "k8s.io/api/core/v1"
	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	RefResolver    *refresolver.RefResolver
	ConditionReady *conditions.Ready
	SqlOpts        []database.SqlOpt
}

// NewUserReconciler creates a new UserReconciler.
func NewUserReconciler(client client.Client, refResolver *refresolver.RefResolver, conditionReady *conditions.Ready,
	dbOpts ...database.SqlOpt) *UserReconciler {
	return &UserReconciler{
		Client:         client,
		RefResolver:    refResolver,
		ConditionReady: conditionReady,
		SqlOpts:        dbOpts,
	}
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling User", "name", req.NamespacedName)

	var user doltv1alpha.User
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	userReconciler := newWrapperUserReconciler(r.Client, r.RefResolver, &user)
	userFinalizer := newWrappedUserFinalizer(r.Client, &user)
	finalizerCtrl := database.NewSqlFinalizer(r.Client, userFinalizer, r.SqlOpts...)
	userCtrl := database.NewSqlReconciler(r.Client, r.ConditionReady, userReconciler, finalizerCtrl, r.SqlOpts...)

	result, err := userCtrl.Reconcile(ctx, &user)
	if err != nil {
		return result, fmt.Errorf("error reconciling in User Controller: %v", err)
	}
	return result, nil
}

type wrappedUserReconciler struct {
	client.Client
	refResolver *refresolver.RefResolver
	user        *doltv1alpha.User
}

// SetupWithManager sets up the UserReconciler with the provided manager.
func (r *UserReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.User{})

	// Sets up a watcher indexer to watch for changes in Secret resources
	// and reconcile them with User and UserList resources based on the UserPasswordSecretFieldPath.
	// The watcher uses a predicate to filter events based on the WatchLabel.
	watcherIndexer := watch.NewWatcherIndexer(mgr, builder, r.Client)
	if err := watcherIndexer.Watch(
		ctx,
		&corev1.Secret{},
		&doltv1alpha.User{},
		&doltv1alpha.UserList{},
		doltv1alpha.UserPasswordSecretFieldPath,
		ctrlbuilder.WithPredicates(
			predicate.PredicateWithLabel(dolt.WatchLabel),
		),
	); err != nil {
		return fmt.Errorf("error watching: %v", err)
	}

	return builder.Complete(r)
}

func newWrapperUserReconciler(client client.Client, refResolver *refresolver.RefResolver,
	user *doltv1alpha.User) database.WrappedReconciler {
	return &wrappedUserReconciler{
		Client:      client,
		refResolver: refResolver,
		user:        user,
	}
}

func (wr *wrappedUserReconciler) Reconcile(ctx context.Context, doltdbClient *sql.Client) error {
	var createUserOpts []sql.CreateUserOpt

	var password string

	if wr.user.Spec.PasswordSecretKeyRef != nil {
		var err error
		password, err = wr.refResolver.SecretKeyRef(ctx, *wr.user.Spec.PasswordSecretKeyRef, wr.user.Namespace)
		if err != nil {
			return fmt.Errorf("error reading user password secret: %v", err)
		}
		createUserOpts = append(createUserOpts, sql.WithIdentifiedBy(password))
	}

	username := wr.user.Username()
	hostname := wr.user.HostnameOrDefault()
	accountName := wr.user.AccountName()

	exists, err := doltdbClient.UserExists(ctx, username, hostname)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error checking if User exists")
	}

	if !exists {
		if err := doltdbClient.CreateUser(ctx, accountName, createUserOpts...); err != nil {
			return fmt.Errorf("error creating User: %v", err)
		}
	} else if password != "" {
		if err := doltdbClient.AlterUser(ctx, accountName, createUserOpts...); err != nil {
			return fmt.Errorf("error altering User: %v", err)
		}
	}
	return nil
}

func (wr *wrappedUserReconciler) PatchStatus(ctx context.Context, patcher conditions.Patcher) error {
	patch := client.MergeFrom(wr.user.DeepCopy())
	patcher(&wr.user.Status)

	if err := wr.Client.Status().Patch(ctx, wr.user, patch); err != nil {
		return fmt.Errorf("error patching User status: %v", err)
	}
	return nil
}

type wrappedUserFinalizer struct {
	client.Client
	user *doltv1alpha.User
}

func newWrappedUserFinalizer(client client.Client, user *doltv1alpha.User) database.WrappedFinalizer {
	return &wrappedUserFinalizer{
		Client: client,
		user:   user,
	}
}

func (wf *wrappedUserFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.user, func(user *doltv1alpha.User) {
		controllerutil.AddFinalizer(user, dolt.UserFinalizerName)
	})
}

func (wf *wrappedUserFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.user, func(user *doltv1alpha.User) {
		controllerutil.RemoveFinalizer(user, dolt.UserFinalizerName)
	})
}

func (wf *wrappedUserFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wf.user, dolt.UserFinalizerName)
}

func (wf *wrappedUserFinalizer) Reconcile(ctx context.Context, doltdbClient *sql.Client) error {
	if err := doltdbClient.DropUser(ctx, wf.user.AccountName()); err != nil {
		return fmt.Errorf("error dropping user in DoltDB: %v", err)
	}
	return nil
}

func (wf *wrappedUserFinalizer) patch(ctx context.Context, user *doltv1alpha.User,
	patchFn func(*doltv1alpha.User)) error {
	patch := client.MergeFrom(user.DeepCopy())
	patchFn(user)

	if err := wf.Client.Patch(ctx, user, patch); err != nil {
		return fmt.Errorf("error removing finalizer to User: %v", err)
	}
	return nil
}
