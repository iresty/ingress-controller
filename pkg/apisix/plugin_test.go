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
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/nettest"
)

type fakeAPISIXPluginSrv struct {
	plugins map[string]string
}

func (srv *fakeAPISIXPluginSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if !strings.HasPrefix(r.URL.Path, "/apisix/admin/plugins") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		list := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(list) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		pluginName := list[len(list)-1]
		if resp, ok := srv.plugins[pluginName]; ok {
			_, _ = w.Write([]byte(resp))
		} else {
			_, _ = w.Write([]byte(errMsg))
		}
		w.WriteHeader(http.StatusOK)
		return
	}

}

func runFakePluginSrv(t *testing.T) *http.Server {
	srv := &fakeAPISIXPluginSrv{
		plugins: testData,
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
	//srv := runFakePluginSrv(t)
	//defer func() {
	//	assert.Nil(t, srv.Shutdown(context.Background()))
	//}()
	//
	//u := url.URL{
	//	Scheme: "http",
	//	Host:   srv.Addr,
	//	Path:   "/apisix/admin",
	//}
	//
	//closedCh := make(chan struct{})
	//close(closedCh)
	//cli := newPluginClient(&cluster{
	//	baseURL:     u.String(),
	//	cli:         http.DefaultClient,
	//	cache:       &dummyCache{},
	//	cacheSynced: closedCh,
	//})

	//for k := range testData {
	//	obj, err := cli.Get(context.Background(), k)
	//	assert.Nil(t, err)
	//	assert.Equal(t, obj.Name, k)
	//	assert.Equal(t, obj.Content, testData[k])
	//}
	//
	//obj, err := cli.Get(context.Background(), "not-a-plugin")
	//assert.Nil(t, err)
	//assert.Equal(t, obj.Content, errMsg)
}
