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

package api

import (
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"

	apirouter "github.com/apache/apisix-ingress-controller/pkg/api/router"
	"github.com/apache/apisix-ingress-controller/pkg/config"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	"github.com/apache/apisix-ingress-controller/pkg/types"
)

// Server represents the API Server in ingress-apisix-controller.
type Server struct {
	router       *gin.Engine
	httpListener net.Listener
	pprofMu      *http.ServeMux
	certFile     string
	keyFile      string
	addr         string
}

// NewServer initializes the API Server.
func NewServer(cfg *config.Config) (*Server, error) {
	httpListener, err := net.Listen("tcp", cfg.HTTPListen)
	if err != nil {
		return nil, err
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())
	apirouter.Mount(router)

	srv := &Server{
		router:       router,
		httpListener: httpListener,
		certFile:     cfg.CertFilePath,
		keyFile:      cfg.KeyFilePath,
		addr:         cfg.HTTPListen,
	}

	if cfg.EnableProfiling {
		srv.pprofMu = new(http.ServeMux)
		srv.pprofMu.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		srv.pprofMu.HandleFunc("/debug/pprof/profile", pprof.Profile)
		srv.pprofMu.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		srv.pprofMu.HandleFunc("/debug/pprof/trace", pprof.Trace)
		srv.pprofMu.HandleFunc("/debug/pprof/", pprof.Index)
		router.GET("/debug/pprof/*profile", gin.WrapF(srv.pprofMu.ServeHTTP))
	}

	return srv, nil
}

// Run launches the API Server.
func (srv *Server) Run(stopCh <-chan struct{}) error {
	go func() {
		<-stopCh
		if err := srv.httpListener.Close(); err != nil {
			log.Errorf("failed to close http listener: %s", err)
		}
	}()

	if srv.keyFile == "" || srv.certFile == "" {
		if err := srv.router.RunListener(srv.httpListener); err != nil && !types.IsUseOfClosedNetConnErr(err) {
			log.Errorf("failed to start API Server: %s", err)
			return err
		}
	} else {
		if err := srv.router.RunTLS(srv.addr, srv.certFile, srv.keyFile); err != nil && !types.IsUseOfClosedNetConnErr(err) {
			log.Errorf("failed to start API Server with TLS: %s", err)
			return err
		}
	}
	return nil
}
