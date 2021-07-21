#!/bin/bash

set -euo pipefail

KNATIVE_NAMESPACE=knative-serving
APISIX_NAMESPACE=ingress-apisix

export KIND_CLUSTER_NAME="ingress-apisix-knative"
kind delete cluster
kind create cluster

echo "Deploying Knative Serving"
KNATIVE_VERSION=v0.23.0
kubectl apply -f https://github.com/knative/serving/releases/download/${KNATIVE_VERSION}/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/${KNATIVE_VERSION}/serving-core.yaml
kubectl patch configmap/config-network -n ${KNATIVE_NAMESPACE} --type merge -p '{"data":{"ingress.class":"apisix"}}'

export KO_DOCKER_REPO=kind.local

kubectl create namespace ${APISIX_NAMESPACE}
echo "Deploying APISIX"
# Should already be installed by Helm, check kubectl to change service type to NodePort
helm install apisix apisix/apisix \
  --set admin.allow.ipList="{0.0.0.0/0}" \
  --namespace ${APISIX_NAMESPACE}
kubectl patch svc apisix-admin --type='json' -p '[{"op":"replace","path":"/spec/type","value":"NodePort"}]' -n=ingress-apisix

echo "Wait for APISIX deployment to be up (timeout=300s)"
kubectl -n ${APISIX_NAMESPACE} wait --timeout=300s --for=condition=Available deployments --all

#kubectl patch svc apisix-admin --type='json' -p '[{"op":"replace","path":"/spec/type","value":"NodePort"}]' -n=${APISIX_NAMESPACE}
#ip=$(kubectl get nodes -lkubernetes.io/hostname!=kind-control-plane -ojsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}' | head -n1)
#port=$(kubectl -n ingress-apisix get svc apisix -ojsonpath='{.spec.ports[?(@.name=="http2")].nodePort}')
export NODE_IP=$(kubectl get nodes --namespace ingress-apisix -o jsonpath="{.items[0].status.addresses[0].address}")
export ADMIN_PORT=$(kubectl get --namespace ingress-apisix -o jsonpath="{.spec.ports[0].nodePort}" services apisix-admin)
export GATEWAY_PORT=$(kubectl get --namespace ingress-apisix -o jsonpath="{.spec.ports[0].nodePort}" services apisix-gateway)
echo
export APISIX_ADMIN_AUTHORITY=${NODE_IP}:${ADMIN_PORT}
export APISIX_GATEWAY_AUTHORITY=${NODE_IP}:${GATEWAY_PORT}
echo "You can connect to APISIX Admin at ${APISIX_ADMIN_AUTHORITY}"
echo "Example usage: 'curl \"http://${APISIX_ADMIN_AUTHORITY}/apisix/admin/services/\" -H 'X-API-KEY: edd1c9f034335f136f87ad84b625c8f1'"

echo "Deploying APISIX ingress controller"
# https://github.com/google/ko#does-ko-work-with-kustomize
# https://github.com/apache/apisix-ingress-controller/blob/master/install.md#kustomize-support
# TODO: add GO_LDFLAGS when build and see whether it can change the version 'unknown' build by ko publish
#kubectl kustomize samples/deploy |
#  sed 's/LoadBalancer/NodePort/g' |
#  ko resolve -f - |
#  kubectl apply -f -

echo "Wait for APISIX ingress controller deployment to be up (timeout=300s)"
kubectl -n ${APISIX_NAMESPACE} wait --timeout=300s --for=condition=Available deployments --all
