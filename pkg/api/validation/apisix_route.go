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

package validation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	v1 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v1"
	"github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2alpha1"
	"github.com/apache/apisix-ingress-controller/pkg/log"

	kwhhttp "github.com/slok/kubewebhook/v2/pkg/http"
	kwhmodel "github.com/slok/kubewebhook/v2/pkg/model"
	kwhvalidating "github.com/slok/kubewebhook/v2/pkg/webhook/validating"
	"github.com/xeipuuv/gojsonschema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewPluginValidatorHandler() http.Handler {
	// Create a validating webhook.
	wh, err := kwhvalidating.NewWebhook(kwhvalidating.WebhookConfig{
		ID:        "apisixRoutes-plugin",
		Validator: pluginValidator,
	})
	if err != nil {
		log.Errorf("failed to create webhook: %s", err)
	}

	h, err := kwhhttp.HandlerFor(kwhhttp.HandlerConfig{Webhook: wh})
	if err != nil {
		log.Errorf("failed to create webhook handle: %s", err)
	}

	return h
}

// ErrNotApisixRoute will be used when the validating object is not ApisixRoute.
var ErrNotApisixRoute = errors.New("object is not ApisixRoute")

var pluginValidator = kwhvalidating.ValidatorFunc(
	func(ctx context.Context, review *kwhmodel.AdmissionReview, object metav1.Object) (result *kwhvalidating.ValidatorResult, err error) {
		valid := true
		var msg []string
		switch ar := object.(type) {
		case *v2alpha1.ApisixRoute:
			for _, h := range ar.Spec.HTTP {
				for _, p := range h.Plugins {
					if p.Enable {
						if err, re := validateSchema(PluginSchema[p.Name], p.Config); err != nil {
							valid = false
							msg = append(msg, fmt.Sprintf("%s plugin's config is invalid\n", p.Name))
							for _, desc := range re {
								msg = append(msg, desc.String())
							}
						}
					}
				}
			}
		case *v1.ApisixRoute:
			for _, r := range ar.Spec.Rules {
				for _, path := range r.Http.Paths {
					for _, p := range path.Plugins {
						if p.Enable {
							if err, re := validateSchema(PluginSchema[p.Name], p.Config); err != nil {
								valid = false
								msg = append(msg, fmt.Sprintf("%s plugin's config is invalid\n", p.Name))
								for _, desc := range re {
									msg = append(msg, desc.String())
								}
							}
						}
					}
				}
			}
		default:
			return &kwhvalidating.ValidatorResult{Valid: false, Message: ErrNotApisixRoute.Error()}, ErrNotApisixRoute
		}

		return &kwhvalidating.ValidatorResult{Valid: valid, Message: strings.Join(msg, "\n")}, nil
	})

func validateSchema(schema string, config interface{}) (error, []gojsonschema.ResultError) {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	configLoader := gojsonschema.NewGoLoader(config)

	result, err := gojsonschema.Validate(schemaLoader, configLoader)
	if err != nil {
		log.Errorf("failed to load and validate the schema: %s", err)
		return err, nil
	}

	if result.Valid() {
		log.Info("the plugin's configLoader is valid")
	} else {
		log.Error("the plugin's configLoader is not valid. see errors:")
		for _, desc := range result.Errors() {
			log.Errorf("- %s\n", desc)
		}
		return fmt.Errorf("invalid plugin config"), result.Errors()
	}

	return nil, nil
}
