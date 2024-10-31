#!/bin/bash

set -euo pipefail

KIND_NETWORK_NAME="kind"
KIND_CIDR=$(docker network inspect -f '{{.IPAM.Config}}' $KIND_NETWORK_NAME | cut -d'{' -f2 | cut -d' ' -f1)
CIDR_PREFIX="172.18"

IP="${CIDR_PREFIX}.0.$1"
HOSTNAME=$2

if grep -q "^$IP\s*$HOSTNAME" /etc/hosts; then
  echo "\"$HOSTNAME\" host already exists in /etc/hosts"
else
  echo "Adding \"$HOSTNAME\" to /etc/hosts"
  sudo -- sh -c -e "printf '# doltdb-operator\n%s\s%s\n' '$IP' '$HOSTNAME' >> /etc/hosts"
fi
