---
title: Install Ingress APISIX on ACK
---

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

This document explains how to install Ingress APISIX on [ali-cloud ACK](https://www.aliyun.com/product/kubernetes).

## Prerequisites

* Create an ACK Service on ali-cloud.
* Download the kube config for your ACK, follow the [introduction](https://www.alibabacloud.com/help/zh/doc-detail/86378.html).
* Install [Helm](https://helm.sh/).
* Clone [Apache APISIX Charts](https://github.com/apache/apisix-helm-chart).
* **Make sure your target namespace exists**, `kubectl` operations thorough this document will be executed in namespace `ingress-apisix`.

## Install APISIX and apisix-ingress-controller

As the data plane of apisix-ingress-controller, [Apache APISIX](http://apisix.apache.org/) can be deployed at the same time using Helm chart.

```shell
cd /path/to/apisix-helm-chart
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
kubectl create ns ingress-apisix
helm install apisix charts/apisix \
  --set gateway.type=LoadBalancer \
  --set ingress-controller.enabled=true \
  --set etcd.persistence.storageClass="alicloud-disk-ssd" \
  --set etcd.persistence.size="20Gi" \
  --namespace ingress-apisix \
kubectl get service --namespace ingress-apisix
```

Five Service resources were created.

* `apisix-gateway`, which processes the real traffic;
* `apisix-admin`, which acts as the control plane to process all the configuration changes.
* `apisix-ingress-controller`, which exposes apisix-ingress-controller's metrics.
* `apisix-etcd` and `apisix-etcd-headless` for etcd service and internal communication.

The gateway service type is set to `LoadBalancer` (See [Access services through SLB](https://www.alibabacloud.com/help/doc-detail/182218.htm) for more details), so that clients can access Apache APISIX through a load balancer. You can find the load balancer ip by running:

```shell
kubectl get service apisix-gateway --namespace ingress-apisix -o jsonpath='{.status.loadBalancer.ingress[].ip}'
```

Another thing should be concerned that the `allow.ipList` field should be customized according to the [Pod CIRD configuration of ACK](https://www.alibabacloud.com/help/en/doc-detail/86500.htm), so that the apisix-ingress-controller instances can access the APISIX instances (resources pushing).

`ACK` PV require min_size is `20Gi`,cluster with `flexVolume` component select `alicloud-disk-ssd`,if with `helm values.yml` configure startup `apisix`,[more helm etcd configure](https://hub.kubeapps.com/charts/bitnami/etcd),configure format sample:

```yaml
etcd:
  persistence:
    storageClass: "alicloud-disk-ssd"
    size: 20Gi
```

Try to create some [resources](https://github.com/apache/apisix-ingress-controller/tree/master/docs/en/latest/concepts) to verify the running status. As a minimalist example, see [proxy-the-httpbin-service](../practices/proxy-the-httpbin-service.md) to learn how to apply resources to drive the apisix-ingress-controller.
