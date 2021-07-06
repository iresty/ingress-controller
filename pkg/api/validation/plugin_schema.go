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

// PluginSchema stores all plugins' schema.
// TODO: add more plugins' schema.
var PluginSchema = map[string]string{
	"api-breaker": `
{
    "type": "object",
    "properties": {
        "break_response_code": {
            "type": "integer",
            "minimum": 200,
            "maximum": 599
        },
        "max_breaker_sec": {
            "type": "integer",
            "minimum": 3,
            "default": 300
        },
        "unhealthy": {
            "type": "object",
            "properties": {
                "http_status": {
                    "type": "array",
                    "minItems": 1,
                    "items": {
                        "type": "integer",
                        "minimum": 500,
                        "maximum": 599
                    },
                    "uniqueItems": true,
                    "default": [500]
                },
                "failures": {
                    "type": "integer",
                    "minimum": 1,
                    "default": 3
                }
            },
            "default": {
                "http_status": [500],
                "failures": 3
            }
        },
        "healthy": {
            "type": "object",
            "properties": {
                "http_status": {
                    "type": "array",
                    "minItems": 1,
                    "items": {
                        "type": "integer",
                        "minimum": 200,
                        "maximum": 499
                    },
                    "uniqueItems": true,
                    "default": [200]
                },
                "successes": {
                    "type": "integer",
                    "minimum": 1,
                    "default": 3
                }
            },
            "default": {
                "http_status": [200],
                "successes": 3
            }
        }
    },
    "required": ["break_response_code"]
}
`,
}
