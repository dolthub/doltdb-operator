# DoltDB Operator

[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](LICENSE.txt)

[DoltDB](https://www.dolthub.com/blog/what-is-dolt/) brings Git-style versioning to your database — branches, diffs, merges, and a complete history of every change. Running it on Kubernetes, however, means managing StatefulSets, replication topology, failover, backups, and configuration by hand.

This operator takes care of all of that. Define your DoltDB cluster as a Kubernetes Custom Resource, and the operator handles provisioning, replication, automatic failover, rolling upgrades, and backups — so you can focus on your data instead of your infrastructure.

## Features

- Provision DoltDB clusters with a single Custom Resource
- Automated primary failover with replication-aware candidate selection
- Single-instance and multi-replica modes with seamless scaling transitions
- Scheduled backups via VolumeSnapshots
- Zero-downtime rolling updates for data-plane upgrades
- Declarative SQL resource management: users, grants, and logical databases
- Install via Helm charts

## Quick Start

### Prerequisites

- Kubernetes 1.28+
- Helm 3.x
- A StorageClass that supports dynamic provisioning
- (Recommended) A service mesh with mTLS (e.g., Istio) for network encryption

### Install with Helm

```sh
# Install CRDs (separately for lifecycle management)
helm install doltdb-operator-crds oci://public.ecr.aws/dolthub/doltdb-operator-crds

# Install the operator
helm install doltdb-operator oci://public.ecr.aws/dolthub/doltdb-operator
```

### Deploy a DoltDB Cluster

```yaml
apiVersion: k8s.dolthub.com/v1alpha
kind: DoltDB
metadata:
  name: my-dolt
spec:
  engineVersion: "2.4.1"
  image: dolthub/dolt
  replicas: 2
  storage:
    size: 10Gi
  server:
    listener:
      host: 0.0.0.0
      port: 3306
      maxConnections: 2048
    cluster:
      remotesAPI:
        port: 50051
  globalConfig:
    commitAuthor:
      name: "dolt operator"
      email: "dolt@operator.local"
  replication:
    enabled: true
    primary:
      podIndex: 0
      automaticFailover: true
```

```sh
kubectl apply -f my-dolt.yaml
```

### Connect to DoltDB

```sh
# Port-forward the primary service
kubectl port-forward svc/my-dolt-primary 3306:3306

# Connect with any MySQL client
mysql -h 127.0.0.1 -P 3306 -u root
```

## Custom Resources

| CRD | Description |
|-----|-------------|
| `DoltDB` | Main cluster resource — manages StatefulSet, Services, ConfigMaps, and replication |
| `Database` | Logical database within a DoltDB cluster |
| `User` | Database user management (references credentials from Kubernetes Secrets) |
| `Grant` | SQL permission grants for users |
| `Snapshot` | Point-in-time volume snapshots for backups |
| `Backup` | On-demand backup to S3, DoltHub, or local storage |
| `BackupSchedule` | Cron-based scheduled backups |

## Roadmap

- **Backup-aware switchover** — prevent primary switchover while a backup is running to avoid unnecessary retries
- **Validating and mutating webhooks** — webhook certificate management for CRD admission control
- **Remote replication** — support for cross-cluster replication via Dolt remotes
- **Backup restore** — declarative restore from S3/DoltHub backups via a `BackupRestore` CRD

## Architecture

The operator follows the standard Kubernetes controller pattern built with [kubebuilder](https://book.kubebuilder.io/). The main `DoltDBReconciler` delegates to specialized sub-reconcilers:

```
DoltDBReconciler
  |-- RBACReconciler        (ServiceAccount, Role, RoleBinding)
  |-- ConfigMapReconciler   (DoltDB server configuration)
  |-- ServiceReconciler     (primary, reader, internal headless services)
  |-- StatefulSetReconciler (pods, rolling updates)
  |-- StorageReconciler     (PVC management)
  |-- StatusReconciler      (condition updates)
  +-- ReplicationReconciler (primary election, failover, switchover)
```

SQL resources (`Database`, `User`, `Grant`) connect directly to DoltDB instances to execute DDL statements, with reconciliation tied to the parent `DoltDB` cluster health.

## Helm Chart Configuration

See [`charts/doltdb-operator/values.yaml`](charts/doltdb-operator/values.yaml) for all available values.

Key options:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Operator container image | `public.ecr.aws/dolthub/doltdb-operator-image` |
| `image.tag` | Operator image tag | Chart appVersion |
| `replicas` | Number of operator replicas | `1` |
| `installCrds` | Install CRDs with the operator chart | `false` |
| `installClusterRole` | Create ClusterRole and ClusterRoleBinding | `true` |
| `leaderElection.id` | Leader election lock name | `k8s.dolthub.com` |

### CRDs Chart

CRDs are distributed as a separate Helm chart (`doltdb-operator-crds`) for independent lifecycle management. This allows upgrading CRDs separately from the operator.

## Network Encryption

The operator does not implement application-level TLS for DoltDB connections. It is designed to run in environments with network-level encryption:

- **Istio** with strict mTLS (recommended)
- **Linkerd**, **Cilium**, or other service mesh / CNI with encryption

Health probes use `--no-tls` because they connect via loopback (`127.0.0.1`) within the same pod.

See [SECURITY.md](SECURITY.md) for more details.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and contribution guidelines.

**Quick commands:**

```sh
make build          # Build the operator binary
make test           # Run unit tests
make lint-fix       # Lint and auto-fix
make cluster        # Create a kind cluster
make tiltci         # Run integration tests
```

### Local Tilt Development

Local development requires Go 1.23.x (for `controller-gen` v0.16.1), Docker, kubectl, Kind 0.32 or newer, and Tilt.

```sh
make local-dev-up    # Create/reuse the cluster and start Tilt in this terminal
```

Press Ctrl+C in the Tilt terminal to stop Tilt; the cluster and workloads remain. To delete them:

```sh
make local-dev-down  # Delete the local cluster and its data
```

The workflow uses a dedicated `doltdb-local` Kind cluster (Kubernetes 1.36.1) with two nodes:

- One Kubernetes control-plane node.
- One worker; `make local-dev-up` labels it `node-role.kubernetes.io/worker` after registration. The operator and DoltDB pods (and Istio, when enabled) run on it.

The operator runs two replicas with leader election. The local DoltDB cluster runs two replicas with automatic replication and failover. Its manifest omits `replication.primary.podIndex`, so a Tilt reapply keeps the primary the operator selected after a failover. Both database pods share the worker, so this exercises pod-level failover but not node loss.

The two DoltDB containers request 3 CPU and 4GiB of memory in total; their memory limits allow up to 8GiB combined. This excludes Istio sidecars, the operator, and Kubernetes components. Allocate additional CPU and memory to Docker/Kind or pods may remain Pending.

Istio is disabled by default. To enable it, put `istioctl` on `PATH` and run:

```sh
make local-dev-up ISTIO_ENABLED=true
```

This installs the Istio minimal profile and enables automatic sidecar injection for the local operator and DoltDB pods. Choose an `istioctl` version compatible with Kubernetes 1.36.1. The workflow does not set a strict mTLS policy. Disabling Istio later does not uninstall it or remove the injection labels; run `make local-dev-down` to start from a clean cluster.

The operator is built from this checkout by default. Setting `LOCAL_DEV_OPERATOR_IMAGE` skips the local build and produces a Tilt warning. For a custom local image, build it and load it into the cluster first; skip `kind create cluster` if `doltdb-local` already exists:

```sh
export LOCAL_DEV_OPERATOR_IMAGE=localhost:5000/doltdb-operator:dev
docker build -t "$LOCAL_DEV_OPERATOR_IMAGE" .
mkdir -p .local
kind create cluster --name doltdb-local --config hack/manifests/kind/kind-local-dev.yaml --kubeconfig .local/kubeconfig
kind load docker-image "$LOCAL_DEV_OPERATOR_IMAGE" --name doltdb-local
make local-dev-up
```

For a registry image, use a pullable immutable digest such as `registry.example/doltdb-operator@sha256:…`.

The local DoltDB manifest sets Dolt engine `2.4.1`. To override it, set `LOCAL_DEV_DOLTDB_ENGINE_VERSION` to a published tag compatible with the existing database data:

```sh
make local-dev-up LOCAL_DEV_DOLTDB_ENGINE_VERSION=<tag>
```

To access the cluster from another terminal, merge the isolated kubeconfig into `kubectl`'s configuration for that shell:

```sh
export KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}:$PWD/.local/kubeconfig"
kubectl config get-contexts
kubectl --context kind-doltdb-local get pods -A
```

This merged setting applies only to this shell and does not edit `~/.kube/config`. Pass `--context kind-doltdb-local` to commands that should target the local cluster.

## License

[BSD 3-Clause License](LICENSE.txt) - Copyright (c) 2025 Electronic Arts Inc.

## Contributing

Before you can contribute, EA must have a Contributor License Agreement (CLA) on file that has been signed by each contributor.
You can sign here: [Go to CLA](https://electronicarts.na1.echosign.com/public/esignWidget?wid=CBFCIBAA3AAABLblqZhByHRvZqmltGtliuExmuV-WNzlaJGPhbSRg2ufuPsM3P0QmILZjLpkGslg24-UJtek*)

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.
