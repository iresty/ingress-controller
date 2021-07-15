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

import (
	"errors"

	"github.com/hashicorp/go-memdb"

	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

var (
	// ErrStillInUse means an object is still in use.
	ErrStillInUse = errors.New("still in use")
	// ErrNotFound is returned when the requested item is not found.
	ErrNotFound = memdb.ErrNotFound
)

type dbCache struct {
	db *memdb.MemDB
}

// NewMemDBCache creates a Cache object backs with a memory DB.
func NewMemDBCache() (Cache, error) {
	db, err := memdb.NewMemDB(_schema)
	if err != nil {
		return nil, err
	}
	return &dbCache{
		db: db,
	}, nil
}

func (c *dbCache) InsertRoute(r *v1.Route) error {
	route := r.DeepCopy()
	return c.insert("route", route)
}

func (c *dbCache) InsertSSL(ssl *v1.Ssl) error {
	return c.insert("ssl", ssl.DeepCopy())
}

func (c *dbCache) InsertUpstream(u *v1.Upstream) error {
	return c.insert("upstream", u.DeepCopy())
}

func (c *dbCache) InsertStreamRoute(sr *v1.StreamRoute) error {
	return c.insert("stream_route", sr.DeepCopy())
}

func (c *dbCache) InsertGlobalRule(gr *v1.GlobalRule) error {
	return c.insert("global_rule", gr.DeepCopy())
}

func (c *dbCache) InsertConsumer(consumer *v1.Consumer) error {
	return c.insert("consumer", consumer.DeepCopy())
}

func (c *dbCache) InsertSchema(schema *v1.Schema) error {
	return c.insert("schema", schema.DeepCopy())
}

func (c *dbCache) insert(table string, obj interface{}) error {
	txn := c.db.Txn(true)
	defer txn.Abort()
	if err := txn.Insert(table, obj); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

func (c *dbCache) GetRoute(id string) (*v1.Route, error) {
	obj, err := c.get("route", id)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Route).DeepCopy(), nil
}

func (c *dbCache) GetSSL(id string) (*v1.Ssl, error) {
	obj, err := c.get("ssl", id)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Ssl).DeepCopy(), nil
}

func (c *dbCache) GetUpstream(id string) (*v1.Upstream, error) {
	obj, err := c.get("upstream", id)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Upstream).DeepCopy(), nil
}

func (c *dbCache) GetStreamRoute(id string) (*v1.StreamRoute, error) {
	obj, err := c.get("stream_route", id)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.StreamRoute).DeepCopy(), nil
}

func (c *dbCache) GetGlobalRule(id string) (*v1.GlobalRule, error) {
	obj, err := c.get("global_rule", id)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.GlobalRule).DeepCopy(), nil
}

func (c *dbCache) GetConsumer(username string) (*v1.Consumer, error) {
	obj, err := c.get("consumer", username)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Consumer).DeepCopy(), nil
}

func (c *dbCache) GetSchema(name string) (*v1.Schema, error) {
	obj, err := c.get("schema", name)
	if err != nil {
		return nil, err
	}
	return obj.(*v1.Schema).DeepCopy(), nil
}

func (c *dbCache) get(table, id string) (interface{}, error) {
	txn := c.db.Txn(false)
	defer txn.Abort()
	obj, err := txn.First(table, "id", id)
	if err != nil {
		if err == memdb.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if obj == nil {
		return nil, ErrNotFound
	}
	return obj, nil
}

func (c *dbCache) ListRoutes() ([]*v1.Route, error) {
	raws, err := c.list("route")
	if err != nil {
		return nil, err
	}
	routes := make([]*v1.Route, 0, len(raws))
	for _, raw := range raws {
		routes = append(routes, raw.(*v1.Route).DeepCopy())
	}
	return routes, nil
}

func (c *dbCache) ListSSL() ([]*v1.Ssl, error) {
	raws, err := c.list("ssl")
	if err != nil {
		return nil, err
	}
	ssl := make([]*v1.Ssl, 0, len(raws))
	for _, raw := range raws {
		ssl = append(ssl, raw.(*v1.Ssl).DeepCopy())
	}
	return ssl, nil
}

func (c *dbCache) ListUpstreams() ([]*v1.Upstream, error) {
	raws, err := c.list("upstream")
	if err != nil {
		return nil, err
	}
	upstreams := make([]*v1.Upstream, 0, len(raws))
	for _, raw := range raws {
		upstreams = append(upstreams, raw.(*v1.Upstream).DeepCopy())
	}
	return upstreams, nil
}

func (c *dbCache) ListStreamRoutes() ([]*v1.StreamRoute, error) {
	raws, err := c.list("stream_route")
	if err != nil {
		return nil, err
	}
	streamRoutes := make([]*v1.StreamRoute, 0, len(raws))
	for _, raw := range raws {
		streamRoutes = append(streamRoutes, raw.(*v1.StreamRoute).DeepCopy())
	}
	return streamRoutes, nil
}

func (c *dbCache) ListGlobalRules() ([]*v1.GlobalRule, error) {
	raws, err := c.list("global_rule")
	if err != nil {
		return nil, err
	}
	globalRules := make([]*v1.GlobalRule, 0, len(raws))
	for _, raw := range raws {
		globalRules = append(globalRules, raw.(*v1.GlobalRule).DeepCopy())
	}
	return globalRules, nil
}

func (c *dbCache) ListConsumers() ([]*v1.Consumer, error) {
	raws, err := c.list("consumer")
	if err != nil {
		return nil, err
	}
	consumers := make([]*v1.Consumer, 0, len(raws))
	for _, raw := range raws {
		consumers = append(consumers, raw.(*v1.Consumer).DeepCopy())
	}
	return consumers, nil
}

func (c *dbCache) list(table string) ([]interface{}, error) {
	txn := c.db.Txn(false)
	defer txn.Abort()
	iter, err := txn.Get(table, "id")
	if err != nil {
		return nil, err
	}
	var objs []interface{}
	for obj := iter.Next(); obj != nil; obj = iter.Next() {
		objs = append(objs, obj)
	}
	return objs, nil
}

func (c *dbCache) DeleteRoute(r *v1.Route) error {
	return c.delete("route", r)
}

func (c *dbCache) DeleteSSL(ssl *v1.Ssl) error {
	return c.delete("ssl", ssl)
}

func (c *dbCache) DeleteUpstream(u *v1.Upstream) error {
	if err := c.checkUpstreamReference(u); err != nil {
		return err
	}
	return c.delete("upstream", u)
}

func (c *dbCache) DeleteStreamRoute(sr *v1.StreamRoute) error {
	return c.delete("stream_route", sr)
}

func (c *dbCache) DeleteGlobalRule(gr *v1.GlobalRule) error {
	return c.delete("global_rule", gr)
}

func (c *dbCache) DeleteConsumer(consumer *v1.Consumer) error {
	return c.delete("consumer", consumer)
}

func (c *dbCache) DeleteSchema(schema *v1.Schema) error {
	return c.delete("schema", schema)
}

func (c *dbCache) delete(table string, obj interface{}) error {
	txn := c.db.Txn(true)
	defer txn.Abort()
	if err := txn.Delete(table, obj); err != nil {
		if err == memdb.ErrNotFound {
			return ErrNotFound
		}
		return err
	}
	txn.Commit()
	return nil
}

func (c *dbCache) checkUpstreamReference(u *v1.Upstream) error {
	// Upstream is referenced by Route.
	txn := c.db.Txn(false)
	defer txn.Abort()
	obj, err := txn.First("route", "upstream_id", u.ID)
	if err != nil && err != memdb.ErrNotFound {
		return err
	}
	if obj != nil {
		return ErrStillInUse
	}

	obj, err = txn.First("stream_route", "upstream_id", u.ID)
	if err != nil && err != memdb.ErrNotFound {
		return err
	}
	if obj != nil {
		return ErrStillInUse
	}
	return nil
}
