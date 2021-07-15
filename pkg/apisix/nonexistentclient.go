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

	"github.com/apache/apisix-ingress-controller/pkg/apisix/cache"
	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

type nonExistentCluster struct {
	embedDummyResourceImplementer
}

func newNonExistentCluster() *nonExistentCluster {
	return &nonExistentCluster{
		embedDummyResourceImplementer{
			route:       &dummyRoute{},
			ssl:         &dummySSL{},
			upstream:    &dummyUpstream{},
			streamRoute: &dummyStreamRoute{},
			globalRule:  &dummyGlobalRule{},
			consumer:    &dummyConsumer{},
			plugin:      &dummyPlugin{},
		},
	}
}

type embedDummyResourceImplementer struct {
	route       Route
	ssl         SSL
	upstream    Upstream
	streamRoute StreamRoute
	globalRule  GlobalRule
	consumer    Consumer
	plugin      Plugin
}

type dummyRoute struct{}

func (f *dummyRoute) Get(_ context.Context, _ string) (*v1.Route, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyRoute) List(_ context.Context) ([]*v1.Route, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyRoute) Create(_ context.Context, _ *v1.Route) (*v1.Route, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyRoute) Delete(_ context.Context, _ *v1.Route) error {
	return ErrClusterNotExist
}

func (f *dummyRoute) Update(_ context.Context, _ *v1.Route) (*v1.Route, error) {
	return nil, ErrClusterNotExist
}

type dummySSL struct{}

func (f *dummySSL) Get(_ context.Context, _ string) (*v1.Ssl, error) {
	return nil, ErrClusterNotExist
}

func (f *dummySSL) List(_ context.Context) ([]*v1.Ssl, error) {
	return nil, ErrClusterNotExist
}

func (f *dummySSL) Create(_ context.Context, _ *v1.Ssl) (*v1.Ssl, error) {
	return nil, ErrClusterNotExist
}

func (f *dummySSL) Delete(_ context.Context, _ *v1.Ssl) error {
	return ErrClusterNotExist
}

func (f *dummySSL) Update(_ context.Context, _ *v1.Ssl) (*v1.Ssl, error) {
	return nil, ErrClusterNotExist
}

type dummyUpstream struct{}

func (f *dummyUpstream) Get(_ context.Context, _ string) (*v1.Upstream, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyUpstream) List(_ context.Context) ([]*v1.Upstream, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyUpstream) Create(_ context.Context, _ *v1.Upstream) (*v1.Upstream, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyUpstream) Delete(_ context.Context, _ *v1.Upstream) error {
	return ErrClusterNotExist
}

func (f *dummyUpstream) Update(_ context.Context, _ *v1.Upstream) (*v1.Upstream, error) {
	return nil, ErrClusterNotExist
}

type dummyStreamRoute struct{}

func (f *dummyStreamRoute) Get(_ context.Context, _ string) (*v1.StreamRoute, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyStreamRoute) List(_ context.Context) ([]*v1.StreamRoute, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyStreamRoute) Create(_ context.Context, _ *v1.StreamRoute) (*v1.StreamRoute, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyStreamRoute) Delete(_ context.Context, _ *v1.StreamRoute) error {
	return ErrClusterNotExist
}

func (f *dummyStreamRoute) Update(_ context.Context, _ *v1.StreamRoute) (*v1.StreamRoute, error) {
	return nil, ErrClusterNotExist
}

type dummyGlobalRule struct{}

func (f *dummyGlobalRule) Get(_ context.Context, _ string) (*v1.GlobalRule, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyGlobalRule) List(_ context.Context) ([]*v1.GlobalRule, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyGlobalRule) Create(_ context.Context, _ *v1.GlobalRule) (*v1.GlobalRule, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyGlobalRule) Delete(_ context.Context, _ *v1.GlobalRule) error {
	return ErrClusterNotExist
}

func (f *dummyGlobalRule) Update(_ context.Context, _ *v1.GlobalRule) (*v1.GlobalRule, error) {
	return nil, ErrClusterNotExist
}

type dummyConsumer struct{}

func (f *dummyConsumer) Get(_ context.Context, _ string) (*v1.Consumer, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyConsumer) List(_ context.Context) ([]*v1.Consumer, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyConsumer) Create(_ context.Context, _ *v1.Consumer) (*v1.Consumer, error) {
	return nil, ErrClusterNotExist
}

func (f *dummyConsumer) Delete(_ context.Context, _ *v1.Consumer) error {
	return ErrClusterNotExist
}

func (f *dummyConsumer) Update(_ context.Context, _ *v1.Consumer) (*v1.Consumer, error) {
	return nil, ErrClusterNotExist
}

type dummyPlugin struct{}

func (f *dummyPlugin) Get(_ context.Context, _ string) (*v1.Schema, error) {
	return nil, ErrClusterNotExist
}

func (nc *nonExistentCluster) Route() Route {
	return nc.route
}

func (nc *nonExistentCluster) SSL() SSL {
	return nc.ssl
}

func (nc *nonExistentCluster) Upstream() Upstream {
	return nc.upstream
}

func (nc *nonExistentCluster) StreamRoute() StreamRoute {
	return nc.streamRoute
}

func (nc *nonExistentCluster) GlobalRule() GlobalRule {
	return nc.globalRule
}

func (nc *nonExistentCluster) Consumer() Consumer {
	return nc.consumer
}

func (nc *nonExistentCluster) Plugin() Plugin {
	return nc.plugin
}

func (nc *nonExistentCluster) HasSynced(_ context.Context) error {
	return nil
}

func (nc *nonExistentCluster) HealthCheck(_ context.Context) error {
	return nil
}

func (nc *nonExistentCluster) String() string {
	return "non-existent cluster"
}

type dummyCache struct{}

var _ cache.Cache = &dummyCache{}

func (c *dummyCache) InsertRoute(_ *v1.Route) error                    { return nil }
func (c *dummyCache) InsertSSL(_ *v1.Ssl) error                        { return nil }
func (c *dummyCache) InsertUpstream(_ *v1.Upstream) error              { return nil }
func (c *dummyCache) InsertStreamRoute(_ *v1.StreamRoute) error        { return nil }
func (c *dummyCache) InsertGlobalRule(_ *v1.GlobalRule) error          { return nil }
func (c *dummyCache) InsertConsumer(_ *v1.Consumer) error              { return nil }
func (c *dummyCache) InsertSchema(_ *v1.Schema) error                  { return nil }
func (c *dummyCache) GetRoute(_ string) (*v1.Route, error)             { return nil, cache.ErrNotFound }
func (c *dummyCache) GetSSL(_ string) (*v1.Ssl, error)                 { return nil, cache.ErrNotFound }
func (c *dummyCache) GetUpstream(_ string) (*v1.Upstream, error)       { return nil, cache.ErrNotFound }
func (c *dummyCache) GetStreamRoute(_ string) (*v1.StreamRoute, error) { return nil, cache.ErrNotFound }
func (c *dummyCache) GetGlobalRule(_ string) (*v1.GlobalRule, error)   { return nil, cache.ErrNotFound }
func (c *dummyCache) GetConsumer(_ string) (*v1.Consumer, error)       { return nil, cache.ErrNotFound }
func (c *dummyCache) GetSchema(_ string) (*v1.Schema, error)           { return nil, cache.ErrNotFound }
func (c *dummyCache) ListRoutes() ([]*v1.Route, error)                 { return nil, nil }
func (c *dummyCache) ListSSL() ([]*v1.Ssl, error)                      { return nil, nil }
func (c *dummyCache) ListUpstreams() ([]*v1.Upstream, error)           { return nil, nil }
func (c *dummyCache) ListStreamRoutes() ([]*v1.StreamRoute, error)     { return nil, nil }
func (c *dummyCache) ListGlobalRules() ([]*v1.GlobalRule, error)       { return nil, nil }
func (c *dummyCache) ListConsumers() ([]*v1.Consumer, error)           { return nil, nil }
func (c *dummyCache) DeleteRoute(_ *v1.Route) error                    { return nil }
func (c *dummyCache) DeleteSSL(_ *v1.Ssl) error                        { return nil }
func (c *dummyCache) DeleteUpstream(_ *v1.Upstream) error              { return nil }
func (c *dummyCache) DeleteStreamRoute(_ *v1.StreamRoute) error        { return nil }
func (c *dummyCache) DeleteGlobalRule(_ *v1.GlobalRule) error          { return nil }
func (c *dummyCache) DeleteConsumer(_ *v1.Consumer) error              { return nil }
func (c *dummyCache) DeleteSchema(_ *v1.Schema) error                  { return nil }
