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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	knativev1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"

	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

func TestTranslateKnativeIngressV1alpha1(t *testing.T) {
	assertions := assert.New(t)
	tr := &translator{}
	ingressList := []*knativev1alpha1.Ingress{
		// 0
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
			},
			Spec: knativev1alpha1.IngressSpec{
				Rules: []knativev1alpha1.IngressRule{
					{},
				},
			},
		},
		// 1
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
			},
			Spec: knativev1alpha1.IngressSpec{
				Rules: []knativev1alpha1.IngressRule{
					{
						Hosts: []string{"my-func.example.com"},
						HTTP: &knativev1alpha1.HTTPIngressRuleValue{
							Paths: []knativev1alpha1.HTTPIngressPath{
								{
									Path: "/",
									AppendHeaders: map[string]string{
										"foo": "bar",
									},
									Splits: []knativev1alpha1.IngressBackendSplit{
										{
											IngressBackend: knativev1alpha1.IngressBackend{
												ServiceNamespace: "foo-ns",
												ServiceName:      "foo-svc",
												ServicePort:      intstr.FromInt(42),
											},
											Percent: 100,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		// 2
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "foo-namespace",
			},
			Spec: knativev1alpha1.IngressSpec{
				Rules: []knativev1alpha1.IngressRule{
					{
						Hosts: []string{"my-func.example.com"},
						HTTP: &knativev1alpha1.HTTPIngressRuleValue{
							Paths: []knativev1alpha1.HTTPIngressPath{
								{
									Path: "/",
									AppendHeaders: map[string]string{
										"foo": "bar",
									},
									Splits: []knativev1alpha1.IngressBackendSplit{
										{
											IngressBackend: knativev1alpha1.IngressBackend{
												ServiceNamespace: "bar-ns",
												ServiceName:      "bar-svc",
												ServicePort:      intstr.FromInt(42),
											},
											Percent: 20,
										},
										{
											IngressBackend: knativev1alpha1.IngressBackend{
												ServiceNamespace: "foo-ns",
												ServiceName:      "foo-svc",
												ServicePort:      intstr.FromInt(42),
											},
											Percent: 100,
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
	t.Run("no ingress returns empty info", func(t *testing.T) {
		translatedCtx, err := tr.translateKnativeIngressV1alpha1(nil)
		assertions.Nil(translatedCtx)
		assertions.Nil(err)
	})
	t.Run("empty ingress returns empty info", func(t *testing.T) {
		translatedCtx, err := tr.translateKnativeIngressV1alpha1(ingressList[0])
		assertions.Equal(&TranslateContext{
			upstreamMap: make(map[string]struct{}),
		}, translatedCtx)
		assertions.Nil(err)
	})
	t.Run("basic knative Ingress resource is parsed", func(t *testing.T) {
		translatedCtx, err := tr.translateKnativeIngressV1alpha1(ingressList[1])
		assertions.NotNil(translatedCtx)
		assertions.Len(translatedCtx.Routes, 1)
		assertions.Len(translatedCtx.Upstreams, 0)
		assertions.Nil(err)
		upstream := translatedCtx.Upstreams[0]
		route := translatedCtx.Routes[0]
		assertions.Equal(route.Name, "knative_ingress_foo-namespace_foo_00")
		assertions.Equal(route.UpstreamId, upstream.ID)
		assertions.Equal(route.Uris, []string{"/foo", "/foo/*"})
		assertions.Equal(route.Hosts, []string{"my-func.example.com"})
		assertions.Equal(route.Plugins["proxy-rewrite"], &v1.RewriteConfig{
			RewriteHeaders: map[string]string{"foo": "bar"},
		})
		// TODO: verify route split plugin
	})
	t.Run("split knative Ingress resource chooses the highest split", func(t *testing.T) {
		translatedCtx, err := tr.translateKnativeIngressV1alpha1(ingressList[1])
		assertions.NotNil(translatedCtx)
		assertions.Len(translatedCtx.Routes, 1)
		assertions.Len(translatedCtx.Upstreams, 0)
		assertions.Nil(err)
		upstream := translatedCtx.Upstreams[0]
		route := translatedCtx.Routes[0]
		assertions.Equal(route.Name, "knative_ingress_foo-namespace_foo_00")
		assertions.Equal(route.UpstreamId, upstream.ID)
		assertions.Equal(route.Uris, []string{"/foo", "/foo/*"})
		assertions.Equal(route.Hosts, []string{"my-func.example.com"})
		assertions.Equal(route.Plugins["proxy-rewrite"], &v1.RewriteConfig{
			RewriteHeaders: map[string]string{"foo": "bar"},
		})
		// TODO: verify route split plugin
	})
}
