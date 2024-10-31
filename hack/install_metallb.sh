#!/bin/bash

set -eo pipefail

CURDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
if [ -z "$METALLB_VERSION" ]; then
  echo "METALLB_VERSION environment variable is mandatory"
  exit 1
fi

kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.9/config/manifests/metallb-native.yaml
kubectl wait --namespace metallb-system \
  --for=condition=ready pod \
  --selector=app=metallb \
  --timeout=90s

# KIND_NETWORK_NAME="kind"
# KIND_CIDR=$(docker network inspect -f '{{.IPAM.Config}}' $KIND_NETWORK_NAME | cut -d'{' -f2 | cut -d' ' -f1)
export CIDR_PREFIX="172.18"

echo "Kind CIDR Prefix: $CIDR_PREFIX"

for f in $CURDIR/manifests/metallb/*; do
  cat $f | envsubst | kubectl apply -f -
done
