package dolt

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterContext represents the context of a Kubernetes cluster, including its namespace,
// namespaced name, associated StatefulSet, and list of pods.
type ClusterContext struct {
	Namespace      string
	NamespacedName string

	client client.Client

	StatefulSet appsv1.StatefulSet
	Pods        []*corev1.Pod
	PodsL       corev1.PodList
}

// Name returns the namespaced name of the cluster.
func (c *ClusterContext) Name() string {
	return c.NamespacedName
}

// ServiceName returns the service name associated with the StatefulSet.
func (c *ClusterContext) ServiceName() string {
	return c.StatefulSet.Spec.ServiceName
}

// NumReplicas returns the number of replicas specified in the StatefulSet.
// If the number of replicas is not specified, it defaults to 1.
func (c *ClusterContext) NumReplicas() int {
	if c.StatefulSet.Spec.Replicas != nil {
		return int(*c.StatefulSet.Spec.Replicas)
	}

	return 1
}

// Instance returns a ClusterInstanceContext for a specific replica index.
func (c *ClusterContext) Instance(i int) ClusterInstanceContext {
	return ClusterInstanceContext{c, i}
}

// ClusterInstanceContext represents the context of a specific instance (replica) within a cluster.
type ClusterInstanceContext struct {
	clusterContext *ClusterContext
	replica        int
}

func (i ClusterInstanceContext) pod() *corev1.Pod {
	return i.clusterContext.Pods[i.replica]
}

func (i ClusterInstanceContext) Port() int {
	return i.clusterContext.port()
}

func (i ClusterInstanceContext) Name() string {
	p := i.pod()
	return p.Namespace + "/" + p.Name
}

func (i ClusterInstanceContext) Hostname() string {
	p := i.pod()
	return p.Name + "." + i.clusterContext.ServiceName() + "." + p.Namespace
}

func (i ClusterInstanceContext) Role() Role {
	p := i.pod()
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok {
		if v == StandbyRoleValue {
			return RoleStandby
		} else if v == PrimaryRoleValue {
			return RolePrimary
		}
	}
	return RoleUnknown
}

func (i ClusterInstanceContext) MarkRolePrimary(ctx context.Context) error {
	p := i.pod()
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok && v == PrimaryRoleValue {
		return nil
	} else {
		p.ObjectMeta.Labels[RoleLabel] = PrimaryRoleValue

		if err := i.clusterContext.client.Update(ctx, p); err != nil {
			return fmt.Errorf("failed to update pod %s to add %s=%s label: %w", i.Name(), RoleLabel, PrimaryRoleValue, err)
		}
		// i.clusterContext.Pods[i.replica] = np
	}
	return nil
}

func (i ClusterInstanceContext) MarkRoleStandby(ctx context.Context) error {
	p := i.pod()
	if v, ok := p.ObjectMeta.Labels[RoleLabel]; ok && v == StandbyRoleValue {
		return nil
	} else {
		p.ObjectMeta.Labels[RoleLabel] = StandbyRoleValue

		if err := i.clusterContext.client.Update(ctx, p); err != nil {
			return fmt.Errorf("failed to update pod %s to add %s=%s label: %w", i.Name(), RoleLabel, StandbyRoleValue, err)
		}
		// i.clusterContext.Pods[i.replica] = np
	}
	return nil
}

func (i ClusterInstanceContext) MarkRoleUnknown(ctx context.Context) error {
	p := i.pod()
	if _, ok := p.ObjectMeta.Labels[RoleLabel]; !ok {
		return nil
	} else {
		delete(p.ObjectMeta.Labels, RoleLabel)

		if err := i.clusterContext.client.Update(ctx, p); err != nil {
			return fmt.Errorf("failed to update pod %s to remove %s label: %w", i.Name(), RoleLabel, err)
		}

		// i.clusterContext.Pods[i.replica] =
	}
	return nil
}

func (i ClusterInstanceContext) Restart(ctx context.Context) error {
	p := i.pod()
	i.clusterContext.Pods
	w, err := pods.Watch(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", p.Name).String(),
	})
	if err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		defer w.Stop()
		defer close(done)
		for {
			select {
			case r := <-w.ResultChan():
				if r.Type == watch.Deleted {
					return
				}
			case <-ctx.Done():
				log.Printf("poll for deletiong of pod %s finished with ctx.Done()", i.Name())
				return
			}
		}
	}()
	log.Printf("deleting pod %s", i.Name())
	err = pods.Delete(ctx, p.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	<-done
	log.Printf("pod %s successfully deleted", i.Name())

	pollInterval := 100 * time.Millisecond
PollPod:
	for {
		time.Sleep(pollInterval)
		if ctx.Err() != nil {
			return fmt.Errorf("error: pod %s did not become Ready after deleting it: %w", i.Name(), ctx.Err())
		}

		p, err := pods.Get(ctx, p.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		for _, c := range p.Status.ContainerStatuses {
			if !c.Ready {
				continue PollPod
			}
		}

		// If we get here, pod exists and all its containers are Ready.
		i.cluster.Pods[i.replica] = p
		return nil
	}
}
