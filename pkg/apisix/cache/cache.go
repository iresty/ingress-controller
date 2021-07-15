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

package cache

import v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"

// Cache defines the necessary behaviors that the cache object should have.
// Note this interface is for APISIX, not for generic purpose, it supports
// standard APISIX resources, i.e. Route, Upstream, and SSL.
// Cache implementations should copy the target objects before/after read/write
// operations for the sake of avoiding data corrupted by other writers.
type Cache interface {
	// InsertRoute adds or updates route to cache.
	InsertRoute(*v1.Route) error
	// InsertSSL adds or updates ssl to cache.
	InsertSSL(*v1.Ssl) error
	// InsertUpstream adds or updates upstream to cache.
	InsertUpstream(*v1.Upstream) error
	// InsertStreamRoute adds or updates stream_route to cache.
	InsertStreamRoute(*v1.StreamRoute) error
	// InsertGlobalRule adds or updates global_rule to cache.
	InsertGlobalRule(*v1.GlobalRule) error
	// InsertConsumer adds or updates consumer to cache.
	InsertConsumer(*v1.Consumer) error
	// InsertSchema adds or updates schema to cache.
	InsertSchema(*v1.Schema) error

	// GetRoute finds the route from cache according to the primary index (id).
	GetRoute(string) (*v1.Route, error)
	// GetSSL finds the ssl from cache according to the primary index (id).
	GetSSL(string) (*v1.Ssl, error)
	// GetUpstream finds the upstream from cache according to the primary index (id).
	GetUpstream(string) (*v1.Upstream, error)
	// GetStreamRoute finds the stream_route from cache according to the primary index (id).
	GetStreamRoute(string) (*v1.StreamRoute, error)
	// GetGlobalRule finds the global_rule from cache according to the primary index (id).
	GetGlobalRule(string) (*v1.GlobalRule, error)
	// GetConsumer finds the consumer from cache according to the primary index (id).
	GetConsumer(string) (*v1.Consumer, error)
	// GetSchema finds the scheme from cache according to the primary index (id).
	GetSchema(string) (*v1.Schema, error)

	// ListRoutes lists all routes in cache.
	ListRoutes() ([]*v1.Route, error)
	// ListSSL lists all ssl objects in cache.
	ListSSL() ([]*v1.Ssl, error)
	// ListUpstreams lists all upstreams in cache.
	ListUpstreams() ([]*v1.Upstream, error)
	// ListStreamRoutes lists all stream_route in cache.
	ListStreamRoutes() ([]*v1.StreamRoute, error)
	// ListGlobalRules lists all global_rule objects in cache.
	ListGlobalRules() ([]*v1.GlobalRule, error)
	// ListConsumers lists all consumer objects in cache.
	ListConsumers() ([]*v1.Consumer, error)

	// DeleteRoute deletes the specified route in cache.
	DeleteRoute(*v1.Route) error
	// DeleteSSL deletes the specified ssl in cache.
	DeleteSSL(*v1.Ssl) error
	// DeleteUpstream deletes the specified upstream in cache.
	DeleteUpstream(*v1.Upstream) error
	// DeleteStreamRoute deletes the specified stream_route in cache.
	DeleteStreamRoute(*v1.StreamRoute) error
	// DeleteGlobalRule deletes the specified stream_route in cache.
	DeleteGlobalRule(*v1.GlobalRule) error
	// DeleteConsumer deletes the specified consumer in cache.
	DeleteConsumer(*v1.Consumer) error
	// DeleteSchema deletes the specified schema in cache.
	DeleteSchema(*v1.Schema) error
}
