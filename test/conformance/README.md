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

# Conformance test for APISIX Ingress Controller
## Dependencies
* install Docker by following command or official instruction
```shell
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io

# create a new user group
sudo usermod -aG docker $USER && newgrp docker
```
* all other dependencies can be installed in a fresh Ubuntu 16.04+ environment by running commands in _./kn-conformance-setup.sh_.
The script installs necessary utilities, Golang, ko, KinD, and Helm. You can also install them manually.
## Clone repo
Suppose the project directory is _~/go/src/fhuzero/apisix-ingress-controller_,
execute the following command to clone the repo and build APISIX Ingress Controller.
```shell
mkdir -p ~/go/src/fhuzero && cd ~/go/src/fhuzero && git clone https://github.com/fhuzero/apisix-ingress-controller.git
cd ~/go/src/fhuzero/apisix-ingress-controller && git switch feat/knative-support
chmod +x utils/setup.sh test/upload-test-images.sh test/e2e-kind.sh test/conformance/kn-conformance-setup.sh
make build
```
## Run conformance test
Run `make knative-integration-test` to start the conformance test.
Note that you need to open another shell to run apisix-ingress-controller locally as follows.
```shell
export NODE_IP=$(kubectl get nodes --namespace ingress-apisix -o jsonpath="{.items[0].status.addresses[0].address}")
export ADMIN_PORT=$(kubectl get --namespace ingress-apisix -o jsonpath="{.spec.ports[0].nodePort}" services apisix-admin)
./apisix-ingress-controller ingress --http-listen :8080  --log-output stderr --apisix-base-url http://${NODE_IP}:${ADMIN_PORT}/apisix/admin --apisix-admin-key edd1c9f034335f136f87ad84b625c8f1
```
