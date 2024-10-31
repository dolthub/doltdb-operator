#!/bin/bash

set -euo pipefail

CURDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

kubectl apply -f $CURDIR/manifests/serviceaccount.yaml
mkdir -p /tmp/dolt-operator
kubectl get secret dolt-operator -o jsonpath="{.data.token}" | base64 -d > /tmp/dolt-operator/token