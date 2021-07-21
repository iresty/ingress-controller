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
package translation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/apache/apisix-ingress-controller/pkg/kube"
	configv1 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v1"
	apisixv1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

func TestTranslateUpstreamConfig(t *testing.T) {
	tr := &translator{}

	au := &configv1.ApisixUpstreamConfig{
		LoadBalancer: nil,
		Scheme:       apisixv1.SchemeGRPC,
	}

	ups, err := tr.TranslateUpstreamConfig(au)
	assert.Nil(t, err, "checking upstream config translating")
	assert.Equal(t, ups.Type, apisixv1.LbRoundRobin)
	assert.Equal(t, ups.Scheme, apisixv1.SchemeGRPC)

	au = &configv1.ApisixUpstreamConfig{
		LoadBalancer: &configv1.LoadBalancer{
			Type:   apisixv1.LbConsistentHash,
			HashOn: apisixv1.HashOnHeader,
			Key:    "user-agent",
		},
		Scheme: apisixv1.SchemeHTTP,
	}
	ups, err = tr.TranslateUpstreamConfig(au)
	assert.Nil(t, err, "checking upstream config translating")
	assert.Equal(t, ups.Type, apisixv1.LbConsistentHash)
	assert.Equal(t, ups.Key, "user-agent")
	assert.Equal(t, ups.HashOn, apisixv1.HashOnHeader)
	assert.Equal(t, ups.Scheme, apisixv1.SchemeHTTP)

	au = &configv1.ApisixUpstreamConfig{
		LoadBalancer: &configv1.LoadBalancer{
			Type:   apisixv1.LbConsistentHash,
			HashOn: apisixv1.HashOnHeader,
			Key:    "user-agent",
		},
		Scheme: "dns",
	}
	_, err = tr.TranslateUpstreamConfig(au)
	assert.Error(t, err, &translateError{
		field:  "scheme",
		reason: "invalid value",
	})

	au = &configv1.ApisixUpstreamConfig{
		LoadBalancer: &configv1.LoadBalancer{
			Type: "hash",
		},
	}
	_, err = tr.TranslateUpstreamConfig(au)
	assert.Error(t, err, &translateError{
		field:  "loadbalancer.type",
		reason: "invalid value",
	})

	au = &configv1.ApisixUpstreamConfig{
		LoadBalancer: &configv1.LoadBalancer{
			Type:   apisixv1.LbConsistentHash,
			HashOn: "arg",
		},
	}
	_, err = tr.TranslateUpstreamConfig(au)
	assert.Error(t, err, &translateError{
		field:  "loadbalancer.hashOn",
		reason: "invalid value",
	})
}

func TestTranslateUpstreamNodes(t *testing.T) {
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc",
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "port1",
					Port: 80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 9080,
					},
				},
				{
					Name: "port2",
					Port: 443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 9443,
					},
				},
			},
		},
	}
	endpoints := &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc",
			Namespace: "test",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Ports: []corev1.EndpointPort{
					{
						Name: "port1",
						Port: 9080,
					},
					{
						Name: "port2",
						Port: 9443,
					},
				},
				Addresses: []corev1.EndpointAddress{
					{IP: "192.168.1.1"},
					{IP: "192.168.1.2"},
				},
			},
		},
	}

	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()

	processCh := make(chan struct{})
	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	go svcInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.HasSynced)

	_, err := client.CoreV1().Services("test").Create(context.Background(), svc, metav1.CreateOptions{})
	assert.Nil(t, err)

	tr := &translator{&TranslatorOptions{
		ServiceLister: svcLister,
	}}
	<-processCh

	nodes, err := tr.TranslateUpstreamNodes(kube.NewEndpoint(endpoints), 10080, nil)
	assert.Nil(t, nodes)
	assert.Equal(t, err, &translateError{
		field:  "service.spec.ports",
		reason: "port not defined",
	})

	nodes, err = tr.TranslateUpstreamNodes(kube.NewEndpoint(endpoints), 80, nil)
	assert.Nil(t, err)
	assert.Equal(t, nodes, apisixv1.UpstreamNodes{
		{
			Host:   "192.168.1.1",
			Port:   9080,
			Weight: 100,
		},
		{
			Host:   "192.168.1.2",
			Port:   9080,
			Weight: 100,
		},
	})

	nodes, err = tr.TranslateUpstreamNodes(kube.NewEndpoint(endpoints), 443, nil)
	assert.Nil(t, err)
	assert.Equal(t, nodes, apisixv1.UpstreamNodes{
		{
			Host:   "192.168.1.1",
			Port:   9443,
			Weight: 100,
		},
		{
			Host:   "192.168.1.2",
			Port:   9443,
			Weight: 100,
		},
	})
}

func TestTranslateUpstreamNodesWithEndpointSlices(t *testing.T) {
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc",
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "port1",
					Port: 80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 9080,
					},
				},
				{
					Name: "port2",
					Port: 443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 9443,
					},
				},
			},
		},
	}
	isTrue := true
	port1 := int32(9080)
	port2 := int32(9443)
	port1Name := "port1"
	port2Name := "port2"
	ep := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "svc",
			Namespace: "test",
			Labels: map[string]string{
				discoveryv1.LabelManagedBy:   "endpointslice-controller.k8s.io",
				discoveryv1.LabelServiceName: "svc",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{
					"192.168.1.1",
					"192.168.1.2",
				},
				Conditions: discoveryv1.EndpointConditions{
					Ready: &isTrue,
				},
			},
		},
		Ports: []discoveryv1.EndpointPort{
			{
				Name: &port1Name,
				Port: &port1,
			},
			{
				Name: &port2Name,
				Port: &port2,
			},
		},
	}

	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()

	processCh := make(chan struct{})
	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	go svcInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.HasSynced)

	_, err := client.CoreV1().Services("test").Create(context.Background(), svc, metav1.CreateOptions{})
	assert.Nil(t, err)

	tr := &translator{&TranslatorOptions{
		ServiceLister: svcLister,
	}}
	<-processCh

	nodes, err := tr.TranslateUpstreamNodes(kube.NewEndpointWithSlice(ep), 10080, nil)
	assert.Nil(t, nodes)
	assert.Equal(t, err, &translateError{
		field:  "service.spec.ports",
		reason: "port not defined",
	})

	nodes, err = tr.TranslateUpstreamNodes(kube.NewEndpointWithSlice(ep), 80, nil)
	assert.Nil(t, err)
	assert.Equal(t, nodes, apisixv1.UpstreamNodes{
		{
			Host:   "192.168.1.1",
			Port:   9080,
			Weight: 100,
		},
		{
			Host:   "192.168.1.2",
			Port:   9080,
			Weight: 100,
		},
	})

	nodes, err = tr.TranslateUpstreamNodes(kube.NewEndpointWithSlice(ep), 443, nil)
	assert.Nil(t, err)
	assert.Equal(t, nodes, apisixv1.UpstreamNodes{
		{
			Host:   "192.168.1.1",
			Port:   9443,
			Weight: 100,
		},
		{
			Host:   "192.168.1.2",
			Port:   9443,
			Weight: 100,
		},
	})
}
