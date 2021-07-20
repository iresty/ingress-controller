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
	"sync"

	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

// APISIX is the unified client tool to communicate with APISIX.
type APISIX interface {
	// Cluster specifies the target cluster to talk.
	Cluster(string) Cluster
	// AddCluster adds a new cluster.
	AddCluster(*ClusterOptions) error
	// UpdateCluster updates an existing cluster.
	UpdateCluster(*ClusterOptions) error
	// ListClusters lists all APISIX clusters.
	ListClusters() []Cluster
}

// Cluster defines specific operations that can be applied in an APISIX
// cluster.
type Cluster interface {
	// Route returns a Route interface that can operate Route resources.
	Route() Route
	// Upstream returns a Upstream interface that can operate Upstream resources.
	Upstream() Upstream
	// SSL returns a SSL interface that can operate SSL resources.
	SSL() SSL
	// StreamRoute returns a StreamRoute interface that can operate StreamRoute resources.
	StreamRoute() StreamRoute
	// GlobalRule returns a GlobalRule interface that can operate GlobalRule resources.
	GlobalRule() GlobalRule
	// String exposes the client information in human readable format.
	String() string
	// HasSynced checks whether all resources in APISIX cluster is synced to cache.
	HasSynced(context.Context) error
	// Consumer returns a Consumer interface that can operate Consumer resources.
	Consumer() Consumer
	// HealthCheck checks apisix cluster health in realtime.
	HealthCheck(context.Context) error
	// Plugin returns a Plugin interface that can operate Plugin resources.
	Plugin() Plugin
	// Schema returns a Schema interface that can fetch schema of APISIX objects.
	Schema() Schema
}

// Route is the specific client interface to take over the create, update,
// list and delete for APISIX Route resource.
type Route interface {
	Get(context.Context, string) (*v1.Route, error)
	List(context.Context) ([]*v1.Route, error)
	Create(context.Context, *v1.Route) (*v1.Route, error)
	Delete(context.Context, *v1.Route) error
	Update(context.Context, *v1.Route) (*v1.Route, error)
}

// SSL is the specific client interface to take over the create, update,
// list and delete for APISIX SSL resource.
type SSL interface {
	Get(context.Context, string) (*v1.Ssl, error)
	List(context.Context) ([]*v1.Ssl, error)
	Create(context.Context, *v1.Ssl) (*v1.Ssl, error)
	Delete(context.Context, *v1.Ssl) error
	Update(context.Context, *v1.Ssl) (*v1.Ssl, error)
}

// Upstream is the specific client interface to take over the create, update,
// list and delete for APISIX Upstream resource.
type Upstream interface {
	Get(context.Context, string) (*v1.Upstream, error)
	List(context.Context) ([]*v1.Upstream, error)
	Create(context.Context, *v1.Upstream) (*v1.Upstream, error)
	Delete(context.Context, *v1.Upstream) error
	Update(context.Context, *v1.Upstream) (*v1.Upstream, error)
}

// StreamRoute is the specific client interface to take over the create, update,
// list and delete for APISIX Stream Route resource.
type StreamRoute interface {
	Get(context.Context, string) (*v1.StreamRoute, error)
	List(context.Context) ([]*v1.StreamRoute, error)
	Create(context.Context, *v1.StreamRoute) (*v1.StreamRoute, error)
	Delete(context.Context, *v1.StreamRoute) error
	Update(context.Context, *v1.StreamRoute) (*v1.StreamRoute, error)
}

// GlobalRule is the specific client interface to take over the create, update,
// list and delete for APISIX Global Rule resource.
type GlobalRule interface {
	Get(context.Context, string) (*v1.GlobalRule, error)
	List(context.Context) ([]*v1.GlobalRule, error)
	Create(context.Context, *v1.GlobalRule) (*v1.GlobalRule, error)
	Delete(context.Context, *v1.GlobalRule) error
	Update(context.Context, *v1.GlobalRule) (*v1.GlobalRule, error)
}

// Consumer is the specific client interface to take over the create, update,
// list and delete for APISIX Consumer resource.
type Consumer interface {
	Get(context.Context, string) (*v1.Consumer, error)
	List(context.Context) ([]*v1.Consumer, error)
	Create(context.Context, *v1.Consumer) (*v1.Consumer, error)
	Delete(context.Context, *v1.Consumer) error
	Update(context.Context, *v1.Consumer) (*v1.Consumer, error)
}

// Plugin is the specific client interface to fetch APISIX Plugin resource.
type Plugin interface {
	List(context.Context) ([]string, error)
}

// Schema is the specific client interface to fetch the schema of APISIX objects.
type Schema interface {
	GetPluginSchema(context.Context, string) (*v1.Schema, error)
}

type apisix struct {
	mu                 sync.RWMutex
	nonExistentCluster Cluster
	clusters           map[string]Cluster
}

// NewClient creates an APISIX client to perform resources change pushing.
func NewClient() (APISIX, error) {
	cli := &apisix{
		nonExistentCluster: newNonExistentCluster(),
		clusters:           make(map[string]Cluster),
	}
	return cli, nil
}

// Cluster implements APISIX.Cluster method.
func (c *apisix) Cluster(name string) Cluster {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cluster, ok := c.clusters[name]
	if !ok {
		return c.nonExistentCluster
	}
	return cluster
}

// ListClusters implements APISIX.ListClusters method.
func (c *apisix) ListClusters() []Cluster {
	c.mu.RLock()
	defer c.mu.RUnlock()
	clusters := make([]Cluster, 0, len(c.clusters))
	for _, cluster := range c.clusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

// AddCluster implements APISIX.AddCluster method.
func (c *apisix) AddCluster(co *ClusterOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.clusters[co.Name]
	if ok {
		return ErrDuplicatedCluster
	}
	cluster, err := newCluster(co)
	if err != nil {
		return err
	}
	c.clusters[co.Name] = cluster
	return nil
}

func (c *apisix) UpdateCluster(co *ClusterOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.clusters[co.Name]; !ok {
		return ErrClusterNotExist
	}

	cluster, err := newCluster(co)
	if err != nil {
		return err
	}

	c.clusters[co.Name] = cluster
	return nil
}
