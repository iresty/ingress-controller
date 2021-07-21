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
	"fmt"
	"github.com/apache/apisix-ingress-controller/pkg/id"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	apisixv1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/intstr"
	knativev1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	"strings"
)

func (t *translator) translateKnativeIngressV1alpha1(ing *knativev1alpha1.Ingress) (*TranslateContext, error) {
	if ing == nil {
		return nil, nil
	}
	ctx := &TranslateContext{
		upstreamMap: make(map[string]struct{}),
	}
	plugins := t.translateAnnotations(ing.Annotations)

	for i, rule := range ing.Spec.Rules {
		hosts := rule.Hosts
		if rule.HTTP == nil {
			continue
		}
		// from https://github.com/knative-sandbox/net-kourier/blob/dd1b827bb5b21c874222c18fc7fc1f3c54e40ee9/pkg/generator/ingress_translator.go#L95
		//ruleName := fmt.Sprintf("(%s/%s).Rules[%d]", ing.Namespace, ing.Name, i)
		//fmt.Printf("In func translateKnativeIngressV1alpha1(): ruleName = %s", ruleName)
		//routes := make([]*route.Route, 0, len(rule.HTTP.Paths))
		for j, httpPath := range rule.HTTP.Paths {
			// Default the path to "/" if none is passed.
			path := httpPath.Path
			if path == "" {
				path = "/"
			}
			var (
				ups *apisixv1.Upstream
				err error
			)
			knativeBackend := knativeSelectSplit(httpPath.Splits)
			serviceName := knativeBackend.IngressBackend.ServiceName
			servicePort := knativeBackend.IngressBackend.ServicePort

			if serviceName != "" {
				ups, err = t.translateUpstreamFromKnativeIngressV1alpha1(ing.Namespace, serviceName, servicePort)
				if err != nil {
					log.Errorw("failed to translate knative ingress backend to upstream",
						zap.Error(err),
						zap.Any("knative ingress", ing),
					)
					return nil, err
				}
				ctx.addUpstream(ups)
			}
			uris := []string{httpPath.Path}
			// httpPath.Path represents a literal prefix to which this rule should apply.
			// As per the specification of Ingress path matching rule:
			// if the last element of the path is a substring of the
			// last element in request path, it is not a match, e.g. /foo/bar
			// matches /foo/bar/baz, but does not match /foo/barbaz.
			// While in APISIX, /foo/bar matches both /foo/bar/baz and
			// /foo/barbaz.
			// In order to be conformant with Ingress specification, here
			// we create two paths here, the first is the path itself
			// (exact match), the other is path + "/*" (prefix match).
			prefix := httpPath.Path
			if strings.HasSuffix(prefix, "/") {
				prefix += "*"
			} else {
				prefix += "/*"
			}
			uris = append(uris, prefix)

			route := apisixv1.NewDefaultRoute()
			// TODO: Figure out a way to name the routes (See Kong ingress controller #834)
			route.Name = composeKnativeIngressRouteName(ing.Namespace, ing.Name, i, j)
			route.ID = id.GenID(route.Name)
			route.Hosts = hosts
			route.Uris = uris

			// add APISIX plugin "proxy-rewrite" to support KIngress' `appendHeaders` property
			var proxyRewritePlugin apisixv1.RewriteConfig
			headers := make(map[string]string)
			for key, value := range knativeBackend.AppendHeaders {
				headers[key] = value
			}
			for key, value := range httpPath.AppendHeaders {
				headers[key] = value
			}
			if len(headers) > 0 {
				proxyRewritePlugin.RewriteHeaders = headers
				plugins["proxy-rewrite"] = proxyRewritePlugin
			}

			if len(plugins) > 0 {
				route.Plugins = *(plugins.DeepCopy())
			}
			if ups != nil {
				route.UpstreamId = ups.ID
			}
			ctx.addRoute(route)
		}
	}
	return ctx, nil
}

func (t *translator) translateUpstreamFromKnativeIngressV1alpha1(namespace string, svcName string, svcPort intstr.IntOrString) (*apisixv1.Upstream, error) {
	var portNumber int32
	if svcPort.Type == intstr.String {
		svc, err := t.ServiceLister.Services(namespace).Get(svcName)
		if err != nil {
			return nil, err
		}
		for _, port := range svc.Spec.Ports {
			if port.Name == svcPort.StrVal {
				portNumber = port.Port
				break
			}
		}
		if portNumber == 0 {
			return nil, &translateError{
				field:  "service",
				reason: "port not found",
			}
		}
	} else {
		portNumber = svcPort.IntVal
	}
	ups, err := t.TranslateUpstream(namespace, svcName, "", portNumber)
	if err != nil {
		return nil, err
	}
	ups.Name = apisixv1.ComposeUpstreamName(namespace, svcName, "", portNumber)
	ups.ID = id.GenID(ups.Name)
	return ups, nil
}

func knativeSelectSplit(splits []knativev1alpha1.IngressBackendSplit) knativev1alpha1.IngressBackendSplit {
	if len(splits) == 0 {
		return knativev1alpha1.IngressBackendSplit{}
	}
	res := splits[0]
	maxPercentage := splits[0].Percent
	if len(splits) == 1 {
		return res
	}
	for i := 1; i < len(splits); i++ {
		if splits[i].Percent > maxPercentage {
			res = splits[i]
			maxPercentage = res.Percent
		}
	}
	return res
}

func composeKnativeIngressRouteName(knativeIngressNamespace, knativeIngressName string, i, j int) string {
	// TODO: convert fmt to buf like to align compose funcs in other files
	return fmt.Sprintf("knative_ingress_%s_%s_%d%d", knativeIngressNamespace, knativeIngressName, i, j)
	//p := make([]byte, 0, len(host)+len(path)+len("knative_ingress")+2)
	//buf := bytes.NewBuffer(p)
	//
	//buf.WriteString("knative_ingress")
	//buf.WriteByte('_')
	//buf.WriteString(host)
	//buf.WriteByte('_')
	//buf.WriteString(path)
	//
	//return buf.String()
}
