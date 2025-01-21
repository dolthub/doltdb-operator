package watch

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Indexer is an interface that extends the client.Object interface and provides
// a method to retrieve an indexing function for a specific field path. This can
// be used to create custom indices for Kubernetes objects, allowing for more
// efficient querying and filtering.
type Indexer interface {
	client.Object
	IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error)
}

// ItemLister is an interface that extends the ctrlclient.ObjectList interface.
// It provides a method to list items of type ctrlclient.Object.
//
// ListItems returns a slice of ctrlclient.Object, representing the items in the list.
type ItemLister interface {
	ctrlclient.ObjectList
	ListItems() []ctrlclient.Object
}

func NewItemListerOfType(itemLister ItemLister) ItemLister {
	itemType := reflect.TypeOf(itemLister).Elem()
	return reflect.New(itemType).Interface().(ItemLister)
}

type WatcherIndexer struct {
	mgr     ctrl.Manager
	builder *builder.Builder
	client  ctrlclient.Client
}

// NewWatcherIndexer creates a new WatcherIndexer
func NewWatcherIndexer(mgr ctrl.Manager, builder *builder.Builder, client ctrlclient.Client) *WatcherIndexer {
	return &WatcherIndexer{
		mgr:     mgr,
		builder: builder,
		client:  client,
	}
}

// Watch sets up a watch on the specified Kubernetes object and indexes a specified field path.
// It uses the provided indexer to create an index function for the field path and registers it
// with the manager's field indexer. It then sets up the watch using the controller-runtime's
// builder, mapping watched objects to reconcile requests using the provided indexer list and field path.
func (rw *WatcherIndexer) Watch(ctx context.Context, obj client.Object, indexer Indexer, indexerList ItemLister,
	indexerFieldPath string, opts ...builder.WatchesOption) error {

	indexerFn, err := indexer.IndexerFuncForFieldPath(indexerFieldPath)
	if err != nil {
		return fmt.Errorf("error getting indexer func: %v", err)
	}
	if err := rw.mgr.GetFieldIndexer().IndexField(ctx, indexer, indexerFieldPath, indexerFn); err != nil {
		return fmt.Errorf("error indexing '%s' field: %v", indexerFieldPath, err)
	}

	rw.builder.Watches(
		obj,
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o ctrlclient.Object) []reconcile.Request {
			return rw.mapWatchedObjectToRequests(ctx, o, indexerList, indexerFieldPath)
		}),
		opts...,
	)
	return nil
}

// Package watch provides functionality for watching and indexing Kubernetes objects
// and mapping them to reconcile requests for a controller.
func (rw *WatcherIndexer) mapWatchedObjectToRequests(ctx context.Context, obj ctrlclient.Object, indexList ItemLister,
	indexerFieldPath string) []reconcile.Request {
	indexersToReconcile := NewItemListerOfType(indexList)
	listOpts := &ctrlclient.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(indexerFieldPath, obj.GetName()),
		Namespace:     obj.GetNamespace(),
	}

	if err := rw.client.List(ctx, indexersToReconcile, listOpts); err != nil {
		return []reconcile.Request{}
	}

	items := indexersToReconcile.ListItems()
	requests := make([]reconcile.Request, len(items))
	for i, item := range items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		}
	}
	return requests
}
