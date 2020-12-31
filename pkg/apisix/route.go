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
	"bytes"
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/api7/ingress-controller/pkg/log"
	v1 "github.com/api7/ingress-controller/pkg/types/apisix/v1"
)

type routeReqBody struct {
	Desc      *string     `json:"desc,omitempty"`
	URI       *string     `json:"uri,omitempty"`
	Host      *string     `json:"host,omitempty"`
	ServiceId *string     `json:"service_id,omitempty"`
	Plugins   *v1.Plugins `json:"plugins,omitempty"`
}

type routeClient struct {
	url  string
	stub *stub
}

func newRouteClient(stub *stub) Route {
	return &routeClient{
		url:  stub.baseURL + "/routes",
		stub: stub,
	}
}

func (r *routeClient) List(ctx context.Context, group string) ([]*v1.Route, error) {
	log.Infow("try to list routes in APISIX", zap.String("url", r.url))

	routeItems, err := r.stub.listResource(ctx, r.url)
	if err != nil {
		log.Errorf("failed to list routes: %s", err)
		return nil, err
	}

	var items []*v1.Route
	for i, item := range routeItems.Node.Items {
		route, err := item.route(group)
		if err != nil {
			log.Errorw("failed to convert route item",
				zap.String("url", r.url),
				zap.String("route_key", item.Key),
				zap.Error(err),
			)
			return nil, err
		}

		items = append(items, route)
		log.Infof("list route #%d, body: %s", i, string(item.Value))
	}

	return items, nil
}

func (r *routeClient) Create(ctx context.Context, obj *v1.Route) (*v1.Route, error) {
	log.Infow("try to create route", zap.String("host", *obj.Host))
	data, err := json.Marshal(routeReqBody{
		Desc:      obj.Name,
		URI:       obj.Path,
		Host:      obj.Host,
		ServiceId: obj.ServiceId,

		Plugins: obj.Plugins,
	})
	if err != nil {
		return nil, err
	}
	resp, err := r.stub.createResource(ctx, r.url, bytes.NewReader(data))
	if err != nil {
		log.Errorf("failed to create route: %s", err)
		return nil, err
	}

	var group string
	if obj.Group != nil {
		group = *obj.Group
	}

	return resp.Item.route(group)
}

func (r *routeClient) Delete(ctx context.Context, obj *v1.Route) error {
	log.Infof("delete route, id:%s", *obj.ID)
	url := r.url + "/" + *obj.ID
	return r.stub.deleteResource(ctx, url)
}

func (r *routeClient) Update(ctx context.Context, obj *v1.Route) error {
	log.Infof("update route, id:%s", *obj.ID)
	body, err := json.Marshal(routeReqBody{
		Desc:      obj.Name,
		Host:      obj.Host,
		URI:       obj.Path,
		ServiceId: obj.ServiceId,
		Plugins:   obj.Plugins,
	})
	if err != nil {
		return err
	}
	url := r.url + "/" + *obj.ID
	return r.stub.updateResource(ctx, url, bytes.NewReader(body))
}
