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
package apisix

import (
	"encoding/json"
	"testing"

	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
	"github.com/stretchr/testify/assert"
)

func TestItemUnmarshalJSON(t *testing.T) {
	var items node
	emptyData := `
{
	"key": "test",
	"nodes": {}
}
`
	err := json.Unmarshal([]byte(emptyData), &items)
	assert.Nil(t, err)

	emptyData = `
{
	"key": "test",
	"nodes": {"a": "b", "c": "d"}
}
`
	err = json.Unmarshal([]byte(emptyData), &items)
	assert.Equal(t, err.Error(), "unexpected non-empty object")

	emptyArray := `
{
	"key": "test",
	"nodes": []
}
`
	err = json.Unmarshal([]byte(emptyArray), &items)
	assert.Nil(t, err)
}

func TestItemConvertRoute(t *testing.T) {
	item := &item{
		Key: "/apisix/routes/001",
		Value: json.RawMessage(`
			{
				"upstream_id": "13",
				"host": "foo.com",
				"uri": "/shop/133/details",
				"name": "unknown",
				"methods": ["GET", "POST"]
			}
		`),
	}

	r, err := item.route()
	assert.Nil(t, err)
	assert.Equal(t, r.UpstreamId, "13")
	assert.Equal(t, r.Host, "foo.com")
	assert.Equal(t, r.Uri, "/shop/133/details")
	assert.Equal(t, r.Methods[0], "GET")
	assert.Equal(t, r.Methods[1], "POST")
	assert.Equal(t, r.Name, "unknown")
}

func TestRouteVarsUnmarshalJSONCompatibility(t *testing.T) {
	var route v1.Route
	data := `{"vars":{}}`
	err := json.Unmarshal([]byte(data), &route)
	assert.Nil(t, err)

	data = `{"vars":{"a":"b"}}`
	err = json.Unmarshal([]byte(data), &route)
	assert.Equal(t, err.Error(), "unexpected non-empty object")

	data = `{"vars":[]}`
	err = json.Unmarshal([]byte(data), &route)
	assert.Nil(t, err)

	data = `{"vars":[["http_a","==","b"]]}`
	err = json.Unmarshal([]byte(data), &route)
	assert.Nil(t, err)
	assert.Equal(t, "http_a", route.Vars[0][0].StrVal)
	assert.Equal(t, "==", route.Vars[0][1].StrVal)
	assert.Equal(t, "b", route.Vars[0][2].StrVal)
}
