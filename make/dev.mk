##@ Development

CLUSTER ?= dolt
KIND_CONFIG ?= hack/manifests/kind/kind.yaml
KIND_CONFIG_CI ?= hack/manifests/kind/kind-ci.yaml

ENV ?= \
	DOLTDB_ENGINE_VERSION=$(DOLTDB_ENGINE_VERSION)

.PHONY: cluster
cluster:
	$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

.PHONY: cluster-ci
cluster-ci:
	if kind get clusters | grep -q "${CLUSTER}"; then \
		echo "Cluster ${CLUSTER} already exists."; \
	else \
		echo "Cluster ${CLUSTER} does not exist. Creating it now..."; \
		$(KIND) create cluster --name $(CLUSTER) --config $(KIND_CONFIG_CI); \
	fi

.PHONY: cluster-delete
cluster-delete:
	$(KIND) delete cluster --name $(CLUSTER)

.PHONY: cluster-ctx
cluster-ctx: ## Sets cluster context.
	$(KUBECTL) config use-context kind-$(CLUSTER)

.PHONY: test-int
test-int: ## Run tests.
	$(ENV) go test ./internal/controller/... -v -ginkgo.v --coverprofile=cover.out --timeout 30m

.PHONY: test
test: ## Run tests.
	CGO_ENABLED=1 go test $$(go list ./...| grep -v /internal/controller) -race -coverprofile cover.out

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@# Clean the generated directories.
	rm -rf charts/doltdb-operator/generated/*
	rm -rf charts/doltdb-operator-crds/generated/crd/*

	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." \
		output:crd:artifacts:config=config/crd/bases \
		output:rbac:artifacts:config=config/rbac \
		output:webhook:artifacts:config=config/webhook

	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." \
		output:crd:artifacts:config=charts/doltdb-operator/generated/crd \
		output:rbac:artifacts:config=charts/doltdb-operator/generated/rbac \
		output:webhook:artifacts:config=charts/doltdb-operator/generated/webhook

	@# Copy CRDs to the standalone CRD chart.
	cp charts/doltdb-operator/generated/crd/* charts/doltdb-operator-crds/generated/crd/

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golines golangci-lint ## Run golangci-lint linter
	$(GOLINES) --max-len=140 --tab-len=2 --dry-run --chain-split-dots --shorten-comments pkg/ internal/ cmd/ api/
	$(GOLANGCI_LINT) run -v --color always
	

.PHONY: lint-fix
lint-fix: golines golangci-lint add-copyright ## Run golangci-lint linter and perform fixes
	$(GOLINES) --max-len=140 --tab-len=2 --chain-split-dots --shorten-comments --write-output pkg/ internal/ cmd/ api/
	$(GOLANGCI_LINT) run --fix -v


.PHONY: add-copyright
add-copyright:
	find . -name "*.go" -type f ! -path "./vendor/*" | while read file; do \
      if ! grep -q "// Copyright (c) 2025 Electronic Arts Inc. All rights reserved." "$$file"; then \
       (echo "// Copyright (c) 2025 Electronic Arts Inc. All rights reserved."; cat "$$file") > "$$file.tmp" && mv "$$file.tmp" "$$file"; \
      fi \
    done

LOCAL_DEV_CLUSTER := doltdb-local
LOCAL_DEV_KUBECONFIG := .local/kubeconfig

ISTIO_VERSION ?= 1.23.6
ISTIOCTL := $(LOCALBIN)/istioctl-$(ISTIO_VERSION)
ISTIO_ENABLED ?= true

.PHONY: install-istio
install-istio: $(ISTIOCTL) ## Install Istio and enable sidecar injection for local workloads.
	$(ISTIOCTL) install --set profile=minimal -y
	kubectl create namespace doltdb-operator-system --dry-run=client -o yaml | kubectl apply -f -
	kubectl label namespace doltdb-operator-system istio-injection=enabled --overwrite
	kubectl label namespace default istio-injection=enabled --overwrite
	@if kubectl get deployment doltdb-operator-controller-manager -n doltdb-operator-system >/dev/null 2>&1; then \
	  kubectl rollout restart deployment doltdb-operator-controller-manager -n doltdb-operator-system; \
	fi
	kubectl delete pod dolt-0 -n default --ignore-not-found

$(ISTIOCTL): $(LOCALBIN)
	mkdir -p .local/istio-download
	cd .local/istio-download && curl -fsSL https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIO_VERSION) sh -
	mv .local/istio-download/istio-$(ISTIO_VERSION)/bin/istioctl $(ISTIOCTL)
	rm -rf .local/istio-download

.PHONY: local-dev-up
local-dev-up: ## Start the dedicated local-dev kind cluster and Tilt environment.
	@go version | grep -Eq '^go version go1\.23\.' || { echo "local dev requires Go 1.23.x for controller-gen v0.16.1" >&2; exit 1; }
	@mkdir -p .local

	kind get clusters | grep -Fxq "$(LOCAL_DEV_CLUSTER)" || kind create cluster --name "$(LOCAL_DEV_CLUSTER)" --config hack/manifests/kind/kind-local-dev.yaml --kubeconfig "$(LOCAL_DEV_KUBECONFIG)"

	kind export kubeconfig --name "$(LOCAL_DEV_CLUSTER)" --kubeconfig "$(LOCAL_DEV_KUBECONFIG)"
	KUBECONFIG="$(LOCAL_DEV_KUBECONFIG)" kubectl label node "$(LOCAL_DEV_CLUSTER)-worker" node-role.kubernetes.io/worker=worker --overwrite
	KUBECONFIG="$(LOCAL_DEV_KUBECONFIG)" ISTIO_ENABLED="$(ISTIO_ENABLED)" tilt up -f Tiltfile.dev

.PHONY: local-dev-down
local-dev-down: ## Delete the dedicated local-dev cluster; stop Tilt with Ctrl+C first.
	kind delete cluster --name "$(LOCAL_DEV_CLUSTER)" --kubeconfig "$(LOCAL_DEV_KUBECONFIG)"
	rm -f "$(LOCAL_DEV_KUBECONFIG)"
	rmdir .local 2>/dev/null || true

.PHONY: tilttest
tilttest:
	make cluster-ci CLUSTER=dolttest cluster-ctx CLUSTER=dolttest
	tilt up

.PHONY: tiltci
tiltci: cluster-ctx
	tilt ci

.PHONY: integration-tests-ci
integration-tests-ci: kind install-tilt-ci install-kustomize-ci cluster-ci cluster-ctx
	tilt ci	
