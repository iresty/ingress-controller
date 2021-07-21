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
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/nettest"

	"github.com/stretchr/testify/assert"
)

type fakeAPISIXSchemaSrv struct {
	schema map[string]string
}

var testData = map[string]string{
	// plugins' schema
	"plugins/key-auth":           `{"$comment":"this is a mark for our injected plugin schema","type":"object","additionalProperties":false,"properties":{"disable":{"type":"boolean"},"header":{"default":"apikey","type":"string"}}}`,
	"plugins/batch-requests":     `{"$comment":"this is a mark for our injected plugin schema","type":"object","additionalProperties":false,"properties":{"disable":{"type":"boolean"}}}`,
	"plugins/ext-plugin-pre-req": `{"properties":{"disable":{"type":"boolean"},"extra_info":{"items":{"type":"string","minLength":1,"maxLength":64},"minItems":1,"type":"array"},"conf":{"items":{"properties":{"value":{"type":"string"},"name":{"type":"string","minLength":1,"maxLength":128}},"type":"object"},"minItems":1,"type":"array"}},"$comment":"this is a mark for our injected plugin schema","type":"object"}`,
}

const errMsg = `{"error_msg":"not found schema"}`

func (srv *fakeAPISIXSchemaSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !strings.HasPrefix(r.URL.Path, "/apisix/admin/schema") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/apisix/admin/schema/"), "/")
		if len(name) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if resp, ok := srv.schema[name]; ok {
			_, _ = w.Write([]byte(resp))
		} else {
			_, _ = w.Write([]byte(errMsg))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

}

func runFakeSchemaSrv(t *testing.T) *http.Server {
	srv := &fakeAPISIXSchemaSrv{
		schema: testData,
	}

	ln, _ := nettest.NewLocalListener("tcp")

	httpSrv := &http.Server{
		Addr:    ln.Addr().String(),
		Handler: srv,
	}

	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Errorf("failed to run http server: %s", err)
		}
	}()

	return httpSrv
}

func TestSchemaClient(t *testing.T) {
	srv := runFakeSchemaSrv(t)
	defer func() {
		assert.Nil(t, srv.Shutdown(context.Background()))
	}()

	u := url.URL{
		Scheme: "http",
		Host:   srv.Addr,
		Path:   "/apisix/admin",
	}

	closedCh := make(chan struct{})
	close(closedCh)
	cli := newSchemaClient(&cluster{
		baseURL:     u.String(),
		cli:         http.DefaultClient,
		cache:       &dummyCache{},
		cacheSynced: closedCh,
	})

	// Test `GetPluginSchema`.
	for name := range testData {
		list := strings.Split(name, "/")
		assert.Greater(t, len(list), 0)

		schema, err := cli.GetPluginSchema(context.Background(), list[len(list)-1])
		assert.Nil(t, err)
		assert.Equal(t, schema.Name, name)
		assert.Equal(t, schema.Content, testData[name])
	}

	// Test getting a non-existent plugin's schema.
	schema, err := cli.GetPluginSchema(context.Background(), "not-a-plugin")
	assert.Nil(t, err)
	assert.Equal(t, schema.Content, errMsg)
}
