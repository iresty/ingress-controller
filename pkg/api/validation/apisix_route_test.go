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
	"testing"

	"github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2alpha1"
	"github.com/stretchr/testify/assert"
)

func Test_validateSchema(t *testing.T) {
	type args struct {
		schema string
		config interface{}
	}
	tests := []struct {
		name     string
		args     args
		expValid bool
	}{
		{
			name: "validating is successes",
			args: args{
				schema: PluginSchema["api-breaker"],
				config: v2alpha1.ApisixRouteHTTPPluginConfig{
					"break_response_code": 200,
				},
			},
			expValid: true,
		},
		{
			name: "validating is failed due to missing required fields",
			args: args{
				schema: PluginSchema["api-breaker"],
				config: v2alpha1.ApisixRouteHTTPPluginConfig{
					"max_breaker_sec": 60,
				},
			},
			expValid: false,
		},
		{
			name: "validating is failed due to invalid break_response_code",
			args: args{
				schema: PluginSchema["api-breaker"],
				config: v2alpha1.ApisixRouteHTTPPluginConfig{
					"break_response_code": 100,
				},
			},
			expValid: false,
		},
		{
			name: "validating is failed due to invalid max_breaker_sec",
			args: args{
				schema: PluginSchema["api-breaker"],
				config: v2alpha1.ApisixRouteHTTPPluginConfig{
					"break_response_code": 200,
					"max_breaker_sec":     2,
				},
			},
			expValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err, re := validateSchema(tt.args.schema, tt.args.config)

			if tt.expValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				// If the validation is executed successfully and it is invalid,
				// the returned ResultError should contain at least one error.
				assert.Greater(t, len(re), 0, "failed to load and validate the schema")
			}
		})
	}
}
