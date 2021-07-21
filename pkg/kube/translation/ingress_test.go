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
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/apache/apisix-ingress-controller/pkg/kube"
	fakeapisix "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned/fake"
	apisixinformers "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/informers/externalversions"
)

var (
	_testSvc = &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
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
	_testEp = &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
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
)

func TestTranslateIngressV1NoBackend(t *testing.T) {
	prefix := networkingv1.PathTypePrefix
	// no backend.
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
								},
							},
						},
					},
				},
			},
		},
	}
	tr := &translator{}
	ctx, err := tr.translateIngressV1(ing)
	assert.Len(t, ctx.Routes, 1)
	assert.Len(t, ctx.Upstreams, 0)
	assert.Nil(t, err)
	assert.Equal(t, ctx.Routes[0].UpstreamId, "")
	assert.Equal(t, ctx.Routes[0].Uris, []string{"/foo", "/foo/*"})
}

func TestTranslateIngressV1BackendWithInvalidService(t *testing.T) {
	prefix := networkingv1.PathTypePrefix
	// no backend.
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{
												Name: "undefined-port",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()
	tr := &translator{
		TranslatorOptions: &TranslatorOptions{
			ServiceLister: svcLister,
		},
	}
	ctx, err := tr.translateIngressV1(ing)
	assert.NotNil(t, err)
	assert.Nil(t, ctx)
	assert.Equal(t, err.Error(), "service \"test-service\" not found")

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

	_, err = client.CoreV1().Services("default").Create(context.Background(), _testSvc, metav1.CreateOptions{})
	assert.Nil(t, err)
	_, err = client.CoreV1().Endpoints("default").Create(context.Background(), _testEp, metav1.CreateOptions{})
	assert.Nil(t, err)

	<-processCh
	ctx, err = tr.translateIngressV1(ing)
	assert.Nil(t, ctx, nil)
	assert.Equal(t, err, &translateError{
		field:  "service",
		reason: "port not found",
	})
}

func TestTranslateIngressV1(t *testing.T) {
	prefix := networkingv1.PathTypePrefix
	// no backend.
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{
												Name: "port1",
											},
										},
									},
								},
								{
									Path: "/bar",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "test-service",
											Port: networkingv1.ServiceBackendPort{
												Number: 443,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()
	epLister, epInformer := kube.NewEndpointListerAndInformer(informersFactory, false)
	apisixClient := fakeapisix.NewSimpleClientset()
	apisixInformersFactory := apisixinformers.NewSharedInformerFactory(apisixClient, 0)
	processCh := make(chan struct{})
	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})
	epInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	go svcInformer.Run(stopCh)
	go epInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.HasSynced)

	_, err := client.CoreV1().Services("default").Create(context.Background(), _testSvc, metav1.CreateOptions{})
	assert.Nil(t, err)
	_, err = client.CoreV1().Endpoints("default").Create(context.Background(), _testEp, metav1.CreateOptions{})
	assert.Nil(t, err)

	tr := &translator{
		TranslatorOptions: &TranslatorOptions{
			ServiceLister:        svcLister,
			EndpointLister:       epLister,
			ApisixUpstreamLister: apisixInformersFactory.Apisix().V1().ApisixUpstreams().Lister(),
		},
	}

	<-processCh
	<-processCh
	ctx, err := tr.translateIngressV1(ing)
	assert.Len(t, ctx.Routes, 2)
	assert.Len(t, ctx.Upstreams, 2)
	assert.Nil(t, err)

	assert.Equal(t, ctx.Routes[0].Uris, []string{"/foo", "/foo/*"})
	assert.Equal(t, ctx.Routes[0].UpstreamId, ctx.Upstreams[0].ID)
	assert.Equal(t, ctx.Routes[0].Host, "apisix.apache.org")
	assert.Equal(t, ctx.Routes[1].Uris, []string{"/bar"})
	assert.Equal(t, ctx.Routes[1].UpstreamId, ctx.Upstreams[1].ID)
	assert.Equal(t, ctx.Routes[1].Host, "apisix.apache.org")

	assert.Equal(t, ctx.Upstreams[0].Type, "roundrobin")
	assert.Equal(t, ctx.Upstreams[0].Scheme, "http")
	assert.Len(t, ctx.Upstreams[0].Nodes, 2)
	assert.Equal(t, ctx.Upstreams[0].Nodes[0].Port, 9080)
	assert.Equal(t, ctx.Upstreams[0].Nodes[0].Host, "192.168.1.1")
	assert.Equal(t, ctx.Upstreams[0].Nodes[1].Port, 9080)
	assert.Equal(t, ctx.Upstreams[0].Nodes[1].Host, "192.168.1.2")

	assert.Equal(t, ctx.Upstreams[1].Type, "roundrobin")
	assert.Equal(t, ctx.Upstreams[1].Scheme, "http")
	assert.Len(t, ctx.Upstreams[1].Nodes, 2)
	assert.Equal(t, ctx.Upstreams[1].Nodes[0].Port, 9443)
	assert.Equal(t, ctx.Upstreams[1].Nodes[0].Host, "192.168.1.1")
	assert.Equal(t, ctx.Upstreams[1].Nodes[1].Port, 9443)
	assert.Equal(t, ctx.Upstreams[1].Nodes[1].Host, "192.168.1.2")
}

func TestTranslateIngressV1beta1NoBackend(t *testing.T) {
	prefix := networkingv1beta1.PathTypePrefix
	// no backend.
	ing := &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: []networkingv1beta1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
								},
							},
						},
					},
				},
			},
		},
	}
	tr := &translator{}
	ctx, err := tr.translateIngressV1beta1(ing)
	assert.Len(t, ctx.Routes, 1)
	assert.Len(t, ctx.Upstreams, 0)
	assert.Nil(t, err)
	assert.Equal(t, ctx.Routes[0].UpstreamId, "")
	assert.Equal(t, ctx.Routes[0].Uris, []string{"/foo", "/foo/*"})
}

func TestTranslateIngressV1beta1BackendWithInvalidService(t *testing.T) {
	prefix := networkingv1beta1.PathTypePrefix
	// no backend.
	ing := &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: []networkingv1beta1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
									Backend: networkingv1beta1.IngressBackend{
										ServiceName: "test-service",
										ServicePort: intstr.IntOrString{
											Type:   intstr.String,
											StrVal: "undefined-port",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()
	tr := &translator{
		TranslatorOptions: &TranslatorOptions{
			ServiceLister: svcLister,
		},
	}
	ctx, err := tr.translateIngressV1beta1(ing)
	assert.NotNil(t, err)
	assert.Nil(t, ctx)
	assert.Equal(t, err.Error(), "service \"test-service\" not found")

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

	_, err = client.CoreV1().Services("default").Create(context.Background(), _testSvc, metav1.CreateOptions{})
	assert.Nil(t, err)
	_, err = client.CoreV1().Endpoints("default").Create(context.Background(), _testEp, metav1.CreateOptions{})
	assert.Nil(t, err)

	<-processCh
	ctx, err = tr.translateIngressV1beta1(ing)
	assert.Nil(t, ctx)
	assert.Equal(t, err, &translateError{
		field:  "service",
		reason: "port not found",
	})
}

func TestTranslateIngressV1beta1(t *testing.T) {
	prefix := networkingv1beta1.PathTypePrefix
	// no backend.
	ing := &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: []networkingv1beta1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
									Backend: networkingv1beta1.IngressBackend{
										ServiceName: "test-service",
										ServicePort: intstr.IntOrString{
											Type:   intstr.String,
											StrVal: "port1",
										},
									},
								},
								{
									Path: "/bar",
									Backend: networkingv1beta1.IngressBackend{
										ServiceName: "test-service",
										ServicePort: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: 443,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()
	epLister, epInformer := kube.NewEndpointListerAndInformer(informersFactory, false)
	apisixClient := fakeapisix.NewSimpleClientset()
	apisixInformersFactory := apisixinformers.NewSharedInformerFactory(apisixClient, 0)
	processCh := make(chan struct{})
	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})
	epInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	go svcInformer.Run(stopCh)
	go epInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.HasSynced)

	_, err := client.CoreV1().Services("default").Create(context.Background(), _testSvc, metav1.CreateOptions{})
	assert.Nil(t, err)
	_, err = client.CoreV1().Endpoints("default").Create(context.Background(), _testEp, metav1.CreateOptions{})
	assert.Nil(t, err)

	tr := &translator{
		TranslatorOptions: &TranslatorOptions{
			ServiceLister:        svcLister,
			EndpointLister:       epLister,
			ApisixUpstreamLister: apisixInformersFactory.Apisix().V1().ApisixUpstreams().Lister(),
		},
	}

	<-processCh
	<-processCh
	ctx, err := tr.translateIngressV1beta1(ing)
	assert.Len(t, ctx.Routes, 2)
	assert.Len(t, ctx.Upstreams, 2)
	assert.Nil(t, err)

	assert.Equal(t, ctx.Routes[0].Uris, []string{"/foo", "/foo/*"})
	assert.Equal(t, ctx.Routes[0].UpstreamId, ctx.Upstreams[0].ID)
	assert.Equal(t, ctx.Routes[0].Host, "apisix.apache.org")
	assert.Equal(t, ctx.Routes[1].Uris, []string{"/bar"})
	assert.Equal(t, ctx.Routes[1].UpstreamId, ctx.Upstreams[1].ID)
	assert.Equal(t, ctx.Routes[1].Host, "apisix.apache.org")

	assert.Equal(t, ctx.Upstreams[0].Type, "roundrobin")
	assert.Equal(t, ctx.Upstreams[0].Scheme, "http")
	assert.Len(t, ctx.Upstreams[0].Nodes, 2)
	assert.Equal(t, ctx.Upstreams[0].Nodes[0].Port, 9080)
	assert.Equal(t, ctx.Upstreams[0].Nodes[0].Host, "192.168.1.1")
	assert.Equal(t, ctx.Upstreams[0].Nodes[1].Port, 9080)
	assert.Equal(t, ctx.Upstreams[0].Nodes[1].Host, "192.168.1.2")

	assert.Equal(t, ctx.Upstreams[1].Type, "roundrobin")
	assert.Equal(t, ctx.Upstreams[1].Scheme, "http")
	assert.Len(t, ctx.Upstreams[1].Nodes, 2)
	assert.Equal(t, ctx.Upstreams[1].Nodes[0].Port, 9443)
	assert.Equal(t, ctx.Upstreams[1].Nodes[0].Host, "192.168.1.1")
	assert.Equal(t, ctx.Upstreams[1].Nodes[1].Port, 9443)
	assert.Equal(t, ctx.Upstreams[1].Nodes[1].Host, "192.168.1.2")
}

func TestTranslateIngressExtensionsV1beta1(t *testing.T) {
	prefix := extensionsv1beta1.PathTypePrefix
	// no backend.
	ing := &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: "test-service",
										ServicePort: intstr.IntOrString{
											Type:   intstr.String,
											StrVal: "port1",
										},
									},
								},
								{
									Path: "/bar",
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: "test-service",
										ServicePort: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: 443,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()
	epLister, epInformer := kube.NewEndpointListerAndInformer(informersFactory, false)
	apisixClient := fakeapisix.NewSimpleClientset()
	apisixInformersFactory := apisixinformers.NewSharedInformerFactory(apisixClient, 0)
	processCh := make(chan struct{})
	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})
	epInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			processCh <- struct{}{}
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	go svcInformer.Run(stopCh)
	go epInformer.Run(stopCh)
	cache.WaitForCacheSync(stopCh, svcInformer.HasSynced)

	_, err := client.CoreV1().Services("default").Create(context.Background(), _testSvc, metav1.CreateOptions{})
	assert.Nil(t, err)
	_, err = client.CoreV1().Endpoints("default").Create(context.Background(), _testEp, metav1.CreateOptions{})
	assert.Nil(t, err)

	tr := &translator{
		TranslatorOptions: &TranslatorOptions{
			ServiceLister:        svcLister,
			EndpointLister:       epLister,
			ApisixUpstreamLister: apisixInformersFactory.Apisix().V1().ApisixUpstreams().Lister(),
		},
	}

	<-processCh
	<-processCh
	ctx, err := tr.translateIngressExtensionsV1beta1(ing)
	assert.Len(t, ctx.Routes, 2)
	assert.Len(t, ctx.Upstreams, 2)
	assert.Nil(t, err)

	assert.Equal(t, ctx.Routes[0].Uris, []string{"/foo", "/foo/*"})
	assert.Equal(t, ctx.Routes[0].UpstreamId, ctx.Upstreams[0].ID)
	assert.Equal(t, ctx.Routes[0].Host, "apisix.apache.org")
	assert.Equal(t, ctx.Routes[1].Uris, []string{"/bar"})
	assert.Equal(t, ctx.Routes[1].UpstreamId, ctx.Upstreams[1].ID)
	assert.Equal(t, ctx.Routes[1].Host, "apisix.apache.org")

	assert.Equal(t, ctx.Upstreams[0].Type, "roundrobin")
	assert.Equal(t, ctx.Upstreams[0].Scheme, "http")
	assert.Len(t, ctx.Upstreams[0].Nodes, 2)
	assert.Equal(t, ctx.Upstreams[0].Nodes[0].Port, 9080)
	assert.Equal(t, ctx.Upstreams[0].Nodes[0].Host, "192.168.1.1")
	assert.Equal(t, ctx.Upstreams[0].Nodes[1].Port, 9080)
	assert.Equal(t, ctx.Upstreams[0].Nodes[1].Host, "192.168.1.2")

	assert.Equal(t, ctx.Upstreams[1].Type, "roundrobin")
	assert.Equal(t, ctx.Upstreams[1].Scheme, "http")
	assert.Len(t, ctx.Upstreams[1].Nodes, 2)
	assert.Equal(t, ctx.Upstreams[1].Nodes[0].Port, 9443)
	assert.Equal(t, ctx.Upstreams[1].Nodes[0].Host, "192.168.1.1")
	assert.Equal(t, ctx.Upstreams[1].Nodes[1].Port, 9443)
	assert.Equal(t, ctx.Upstreams[1].Nodes[1].Host, "192.168.1.2")
}

func TestTranslateIngressExtensionsV1beta1BackendWithInvalidService(t *testing.T) {
	prefix := extensionsv1beta1.PathTypePrefix
	// no backend.
	ing := &extensionsv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: extensionsv1beta1.IngressSpec{
			Rules: []extensionsv1beta1.IngressRule{
				{
					Host: "apisix.apache.org",
					IngressRuleValue: extensionsv1beta1.IngressRuleValue{
						HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
							Paths: []extensionsv1beta1.HTTPIngressPath{
								{
									Path:     "/foo",
									PathType: &prefix,
									Backend: extensionsv1beta1.IngressBackend{
										ServiceName: "test-service",
										ServicePort: intstr.IntOrString{
											Type:   intstr.String,
											StrVal: "undefined-port",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset()
	informersFactory := informers.NewSharedInformerFactory(client, 0)
	svcInformer := informersFactory.Core().V1().Services().Informer()
	svcLister := informersFactory.Core().V1().Services().Lister()
	tr := &translator{
		TranslatorOptions: &TranslatorOptions{
			ServiceLister: svcLister,
		},
	}
	ctx, err := tr.translateIngressExtensionsV1beta1(ing)
	assert.Nil(t, ctx)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "service \"test-service\" not found")

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

	_, err = client.CoreV1().Services("default").Create(context.Background(), _testSvc, metav1.CreateOptions{})
	assert.Nil(t, err)
	_, err = client.CoreV1().Endpoints("default").Create(context.Background(), _testEp, metav1.CreateOptions{})
	assert.Nil(t, err)

	<-processCh
	ctx, err = tr.translateIngressExtensionsV1beta1(ing)
	assert.Nil(t, ctx)
	assert.Equal(t, err, &translateError{
		field:  "service",
		reason: "port not found",
	})
}
