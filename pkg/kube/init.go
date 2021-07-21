// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package kube

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"github.com/apache/apisix-ingress-controller/pkg/config"
	clientset "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
	"github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/informers/externalversions"
	knativeclient "knative.dev/networking/pkg/client/clientset/versioned"
	knativeinformer "knative.dev/networking/pkg/client/informers/externalversions"
)

// KubeClient contains some objects used to communicate with Kubernetes API Server.
type KubeClient struct {
	cfg *config.Config

	// Client is the object used to operate Kubernetes builtin resources.
	Client kubernetes.Interface
	// APISIXClient is the object used to operate resources under apisix.apache.org group.
	APISIXClient clientset.Interface
	// KnativeClient is the object used to operate Knative builtin resources.
	KnativeClient knativeclient.Interface
}

// NewKubeClient creates a high-level Kubernetes client.
func NewKubeClient(cfg *config.Config) (*KubeClient, error) {
	restConfig, err := BuildRestConfig(cfg.Kubernetes.Kubeconfig, "")
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	apisixKubeClient, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	knativeClient, err := knativeclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &KubeClient{
		cfg:           cfg,
		Client:        kubeClient,
		APISIXClient:  apisixKubeClient,
		KnativeClient: knativeClient,
	}, nil
}

// NewSharedIndexInformerFactory returns an index informer factory object used to watch and
// list Kubernetes builtin resources.
func (k *KubeClient) NewSharedIndexInformerFactory() informers.SharedInformerFactory {
	return informers.NewSharedInformerFactory(k.Client, k.cfg.Kubernetes.ResyncInterval.Duration)
}

// NewAPISIXSharedIndexInformerFactory returns an index informer factory object used to watch
// and list Kubernetes resources in apisix.apache.org group.
func (k *KubeClient) NewAPISIXSharedIndexInformerFactory() externalversions.SharedInformerFactory {
	return externalversions.NewSharedInformerFactory(k.APISIXClient, k.cfg.Kubernetes.ResyncInterval.Duration)
}

// KnativeSharedIndexInformerFactory is the index informer factory object used to watch
// and list Knative builtin resources.
func (k *KubeClient) NewKnativeSharedIndexInformerFactory() knativeinformer.SharedInformerFactory {
	return knativeinformer.NewSharedInformerFactory(k.KnativeClient, k.cfg.Kubernetes.ResyncInterval.Duration)
}
