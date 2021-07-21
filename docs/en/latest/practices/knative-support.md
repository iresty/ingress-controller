<!--
#
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
-->

# Support for Knative Serving
APISIX Ingress Controller can serve as an Ingress for [Knative Serving](https://knative.dev/).
APISIX Ingress Controller is an alternative for the Istio ingress as the network layer of Knative Serving.

## Getting started
[Hello World - Go](https://knative.dev/docs/serving/samples/hello-world/helloworld-go/#hello-world-go) is a
code sample from Knative. You can use APISIX Ingress Controller with Knative in the sample as follows.
### Preliminary
* [KinD](https://kind.sigs.k8s.io) installed
* create a KinD cluster named "ingress-apisix-knative" and create "ingress-apisix" namespace
```shell
export KIND_CLUSTER_NAME="ingress-apisix-knative"
kind delete cluster
kind create cluster
kubectl create ns ingress-apisix
```
* deploy APISIX in the KinD cluster
```shell
helm install apisix apisix/apisix \
  --set admin.allow.ipList="{0.0.0.0/0}" \
  --namespace ingress-apisix
# local deployment of APISIX Ingress Controller requires apisix-admin proxy service to be of NodePort type
kubectl patch svc apisix-admin --type='json' -p '[{"op":"replace","path":"/spec/type","value":"NodePort"}]' -n=ingress-apisix
```
* build APISIX Ingress Controller and deploy it locally
```shell
cd ${PROJECT_ROOT} && make build

export ADMIN_NODE_PORT=$(kubectl get --namespace ingress-apisix -o jsonpath="{.spec.ports[0].nodePort}" services apisix-admin)
export NODE_IP=$(kubectl get nodes --namespace ingress-apisix -o jsonpath="{.items[0].status.addresses[0].address}")
echo http://$NODE_IP:$ADMIN_NODE_PORT
./apisix-ingress-controller ingress --http-listen :8080  --log-output stderr \
  --apisix-base-url http://$NODE_IP:$ADMIN_NODE_PORT/apisix/admin \
  --apisix-admin-key edd1c9f034335f136f87ad84b625c8f1 \
  --default-apisix-cluster-name ingress-apisix-knative
```
Open a new shell to continue this tutorial if `apisix-ingress-controller` command stays in the foreground. 
### Hello-world app
* Install Knative Serving, ideally without Istio:
```shell
kubectl apply -f https://github.com/knative/serving/releases/download/v0.24.0/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/v0.24.0/serving-core.yaml
```
* Configure ingress class
Configure Knative Serving to use the proper "ingress.class":
```shell
kubectl patch configmap/config-network \
  -n knative-serving \
  --type merge \
  -p '{"data":{"ingress.class":"apisix"}}'
```
* (OPTIONAL) Set your desired domain (replace 127.0.0.1.nip.io to your preferred domain):
```shell
kubectl patch configmap/config-domain \
-n knative-serving \
--type merge \
-p '{"data":{"127.0.0.1.nip.io":""}}'
```
* (OPTIONAL) Deploy a sample hello world app:
```shell
cat <<-EOF | kubectl apply -f -
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: helloworld-go
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-samples/helloworld-go
        env:
        - name: TARGET
          value: Go Sample v1
EOF
```
* (OPTIONAL) For testing purposes, you can use port-forwarding to make requests to APISIX from your machine:
```shell
kubectl port-forward --namespace ingress-apisix $(kubectl get pod -n ingress-apisix -l "app.kubernetes.io/name=apisix" --output=jsonpath="{.items[0].metadata.name}") 9080:9080
```
Open a third shell if the `kubectl port-forward` command stays in the foreground. Then you can see "Hello Go sample v1!" by entering the following command. 
```shell
curl -v -H "Host: helloworld-go.default.127.0.0.1.nip.io" http://localhost:9080
```