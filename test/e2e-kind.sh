#!/usr/bin/env bash

# This script runs e2e tests on a local kind environment.

set -euo pipefail

APISIX_NAMESPACE=ingress-apisix

export KO_DOCKER_REPO=kind.local
export KIND_CLUSTER_NAME="ingress-apisix-knative"
$(dirname $0)/upload-test-images.sh

echo ">> Setup test resources"
# TODO: check whether .yaml files under test/config need to be revised
ko apply -f test/config

#ip=$(kubectl get nodes -lkubernetes.io/hostname!=kind-control-plane -ojsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | head -n1)
export NODE_IP=$(kubectl get nodes --namespace ${APISIX_NAMESPACE} -o jsonpath="{.items[0].status.addresses[0].address}")
echo
export "GATEWAY_OVERRIDE=apisix"
export "GATEWAY_NAMESPACE_OVERRIDE=${APISIX_NAMESPACE}"

echo ">> Running conformance tests"
# timeout is 1m to failfast since the test always timed out
go test -count=1 -short -timeout=5m -tags=e2e -test.v ./test/conformance/... ./test/e2eknative/... \
  --ingressendpoint=${NODE_IP} \
  --ingressClass=apisix
