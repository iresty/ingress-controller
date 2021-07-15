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
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/nettest"

	"github.com/stretchr/testify/assert"
)

type fakeAPISIXPluginSrv struct {
	plugin map[string]json.RawMessage
}

func (srv *fakeAPISIXPluginSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !strings.HasPrefix(r.URL.Path, "/apisix/admin/plugins") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		resp := fakeListResp{
			Count: strconv.Itoa(len(srv.plugin)),
			Node: fakeNode{
				Key: "/apisix/plugins",
			},
		}
		var keys []string
		for key := range srv.plugin {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			resp.Node.Items = append(resp.Node.Items, fakeItem{
				Key:   key,
				Value: srv.plugin[key],
			})
		}
		w.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(resp)
		_, _ = w.Write(data)
		return
	}

}

func runFakePluginSrv(t *testing.T) *http.Server {
	srv := &fakeAPISIXPluginSrv{
		plugin: make(map[string]json.RawMessage),
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

func TestPluginClient(t *testing.T) {
	srv := runFakePluginSrv(t)
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
	cli := newPluginClient(&cluster{
		baseURL:     u.String(),
		cli:         http.DefaultClient,
		cache:       &dummyCache{},
		cacheSynced: closedCh,
	})

	// Get
	obj, err := cli.Get(context.Background(), "key-auth")
	assert.Nil(t, err)
	assert.Equal(t, obj.Name, "key-auth")
	//
	//obj, err = cli.Create(context.Background(), &v1.Route{
	//	Metadata: v1.Metadata{
	//		ID:   "2",
	//		Name: "test",
	//	},
	//	Host:       "www.foo.com",
	//	Uri:        "/bar",
	//	UpstreamId: "1",
	//})
	//assert.Nil(t, err)
	//assert.Equal(t, obj.ID, "2")

}
