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
	"github.com/apache/apisix-ingress-controller/pkg/id"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
	"go.uber.org/zap"
)

type pluginClient struct {
	url     string
	cluster *cluster
}

func newPluginClient(c *cluster) Plugin {
	return &pluginClient{
		url:     c.baseURL + "/plugins",
		cluster: c,
	}
}

// Get returns the plugin's schema.
func (p *pluginClient) Get(ctx context.Context, name string) (*v1.Schema, error) {
	log.Debugw("try to look up plugin's schema",
		zap.String("name", name),
		zap.String("url", p.url),
		zap.String("cluster", "default"),
	)
	sid := id.GenID(name)
	schema, err := p.cluster.cache.GetSchema(sid)
	if err == nil {
		return schema, nil
	}
	if err == cache.ErrNotFound {
		log.Debugw("failed to find plugin's schema in cache, will try to lookup from APISIX",
			zap.String("name", name),
			zap.Error(err),
		)
	} else {
		log.Errorw("failed to find plugin's schema in cache, will try to lookup from APISIX",
			zap.String("name", name),
			zap.Error(err),
		)
	}

	url := p.url + "/" + name
	content, err := p.cluster.getSchema(ctx, url)
	if err != nil {
		log.Errorw("failed to get plugin schema from APISIX",
			zap.String("name", name),
			zap.String("url", url),
			zap.String("cluster", "default"),
			zap.Error(err),
		)
		return nil, err
	}

	schema = &v1.Schema{
		Name:    name,
		Content: content,
	}
	if err := p.cluster.cache.InsertSchema(schema); err != nil {
		log.Errorf("failed to reflect schema create to cache: %s", err)
		return nil, err
	}
	return schema, nil
}
