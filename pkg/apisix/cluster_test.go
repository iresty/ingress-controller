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
	"testing"

	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"

	"github.com/stretchr/testify/assert"
)

func TestAddCluster(t *testing.T) {
	apisix, err := NewClient()
	assert.Nil(t, err)

	err = apisix.AddCluster(&ClusterOptions{
		BaseURL: "http://service1:9080/apisix/admin",
	})
	assert.Nil(t, err)

	clusters := apisix.ListClusters()
	assert.Len(t, clusters, 1)

	err = apisix.AddCluster(&ClusterOptions{
		Name:    "service2",
		BaseURL: "http://service2:9080/apisix/admin",
	})
	assert.Nil(t, err)

	err = apisix.AddCluster(&ClusterOptions{
		Name:     "service2",
		AdminKey: "http://service3:9080/apisix/admin",
	})
	assert.Equal(t, ErrDuplicatedCluster, err)

	clusters = apisix.ListClusters()
	assert.Len(t, clusters, 2)
}

func TestNonExistentCluster(t *testing.T) {
	apisix, err := NewClient()
	assert.Nil(t, err)

	err = apisix.AddCluster(&ClusterOptions{
		BaseURL: "http://service1:9080/apisix/admin",
	})
	assert.Nil(t, err)

	_, err = apisix.Cluster("non-existent-cluster").Route().List(context.Background())
	assert.Equal(t, ErrClusterNotExist, err)
	_, err = apisix.Cluster("non-existent-cluster").Route().Create(context.Background(), &v1.Route{})
	assert.Equal(t, ErrClusterNotExist, err)
	_, err = apisix.Cluster("non-existent-cluster").Route().Update(context.Background(), &v1.Route{})
	assert.Equal(t, ErrClusterNotExist, err)
	err = apisix.Cluster("non-existent-cluster").Route().Delete(context.Background(), &v1.Route{})
	assert.Equal(t, ErrClusterNotExist, err)

	_, err = apisix.Cluster("non-existent-cluster").Upstream().List(context.Background())
	assert.Equal(t, ErrClusterNotExist, err)
	_, err = apisix.Cluster("non-existent-cluster").Upstream().Create(context.Background(), &v1.Upstream{})
	assert.Equal(t, ErrClusterNotExist, err)
	_, err = apisix.Cluster("non-existent-cluster").Upstream().Update(context.Background(), &v1.Upstream{})
	assert.Equal(t, ErrClusterNotExist, err)
	err = apisix.Cluster("non-existent-cluster").Upstream().Delete(context.Background(), &v1.Upstream{})
	assert.Equal(t, ErrClusterNotExist, err)

	_, err = apisix.Cluster("non-existent-cluster").SSL().List(context.Background())
	assert.Equal(t, ErrClusterNotExist, err)
	_, err = apisix.Cluster("non-existent-cluster").SSL().Create(context.Background(), &v1.Ssl{})
	assert.Equal(t, ErrClusterNotExist, err)
	_, err = apisix.Cluster("non-existent-cluster").SSL().Update(context.Background(), &v1.Ssl{})
	assert.Equal(t, ErrClusterNotExist, err)
	err = apisix.Cluster("non-existent-cluster").SSL().Delete(context.Background(), &v1.Ssl{})
	assert.Equal(t, ErrClusterNotExist, err)
}
