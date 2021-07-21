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
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

func TestMemDBCacheRoute(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	r1 := &v1.Route{
		Metadata: v1.Metadata{
			ID:   "1",
			Name: "abc",
		},
	}
	assert.Nil(t, c.InsertRoute(r1), "inserting route 1")

	r, err := c.GetRoute("1")
	assert.Nil(t, err)
	assert.Equal(t, r1, r)

	r2 := &v1.Route{
		Metadata: v1.Metadata{
			ID:   "2",
			Name: "def",
		},
	}
	r3 := &v1.Route{
		Metadata: v1.Metadata{
			ID:   "3",
			Name: "ghi",
		},
	}
	assert.Nil(t, c.InsertRoute(r2), "inserting route r2")
	assert.Nil(t, c.InsertRoute(r3), "inserting route r3")

	r, err = c.GetRoute("3")
	assert.Nil(t, err)
	assert.Equal(t, r3, r)

	assert.Nil(t, c.DeleteRoute(r3), "delete route r3")

	routes, err := c.ListRoutes()
	assert.Nil(t, err, "listing routes")

	if routes[0].Name > routes[1].Name {
		routes[0], routes[1] = routes[1], routes[0]
	}
	assert.Equal(t, routes[0], r1)
	assert.Equal(t, routes[1], r2)

	r4 := &v1.Route{
		Metadata: v1.Metadata{
			ID:   "4",
			Name: "name4",
		},
	}
	assert.Error(t, ErrNotFound, c.DeleteRoute(r4))
}

func TestMemDBCacheSSL(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	s1 := &v1.Ssl{
		ID: "abc",
	}
	assert.Nil(t, c.InsertSSL(s1), "inserting ssl 1")

	s, err := c.GetSSL("abc")
	assert.Nil(t, err)
	assert.Equal(t, s1, s)

	s2 := &v1.Ssl{
		ID: "def",
	}
	s3 := &v1.Ssl{
		ID: "ghi",
	}
	assert.Nil(t, c.InsertSSL(s2), "inserting ssl 2")
	assert.Nil(t, c.InsertSSL(s3), "inserting ssl 3")

	s, err = c.GetSSL("ghi")
	assert.Nil(t, err)
	assert.Equal(t, s3, s)

	assert.Nil(t, c.DeleteSSL(s3), "delete ssl 3")

	ssl, err := c.ListSSL()
	assert.Nil(t, err, "listing ssl")

	if ssl[0].ID > ssl[1].ID {
		ssl[0], ssl[1] = ssl[1], ssl[0]
	}
	assert.Equal(t, ssl[0], s1)
	assert.Equal(t, ssl[1], s2)

	s4 := &v1.Ssl{
		ID: "id4",
	}
	assert.Error(t, ErrNotFound, c.DeleteSSL(s4))
}

func TestMemDBCacheUpstream(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	u1 := &v1.Upstream{
		Metadata: v1.Metadata{
			ID:   "1",
			Name: "abc",
		},
	}
	err = c.InsertUpstream(u1)
	assert.Nil(t, err, "inserting upstream 1")

	u, err := c.GetUpstream("1")
	assert.Nil(t, err)
	assert.Equal(t, u1, u)

	u2 := &v1.Upstream{
		Metadata: v1.Metadata{
			Name: "def",
			ID:   "2",
		},
	}
	u3 := &v1.Upstream{
		Metadata: v1.Metadata{
			Name: "ghi",
			ID:   "3",
		},
	}
	assert.Nil(t, c.InsertUpstream(u2), "inserting upstream 2")
	assert.Nil(t, c.InsertUpstream(u3), "inserting upstream 3")

	u, err = c.GetUpstream("3")
	assert.Nil(t, err)
	assert.Equal(t, u3, u)

	assert.Nil(t, c.DeleteUpstream(u3), "delete upstream 3")

	upstreams, err := c.ListUpstreams()
	assert.Nil(t, err, "listing upstreams")

	if upstreams[0].Name > upstreams[1].Name {
		upstreams[0], upstreams[1] = upstreams[1], upstreams[0]
	}
	assert.Equal(t, upstreams[0], u1)
	assert.Equal(t, upstreams[1], u2)

	u4 := &v1.Upstream{
		Metadata: v1.Metadata{
			Name: "name4",
			ID:   "4",
		},
	}
	assert.Error(t, ErrNotFound, c.DeleteUpstream(u4))
}

func TestMemDBCacheReference(t *testing.T) {
	r := &v1.Route{
		Metadata: v1.Metadata{
			Name: "route",
			ID:   "1",
		},
		UpstreamId: "1",
	}
	u := &v1.Upstream{
		Metadata: v1.Metadata{
			ID:   "1",
			Name: "upstream",
		},
	}
	u2 := &v1.Upstream{
		Metadata: v1.Metadata{
			ID:   "2",
			Name: "upstream",
		},
	}
	sr := &v1.StreamRoute{
		ID:         "1",
		UpstreamId: "2",
	}

	db, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")
	assert.Nil(t, db.InsertRoute(r))
	assert.Nil(t, db.InsertUpstream(u))
	assert.Nil(t, db.InsertStreamRoute(sr))
	assert.Nil(t, db.InsertUpstream(u2))

	assert.Error(t, ErrStillInUse, db.DeleteUpstream(u))
	assert.Error(t, ErrStillInUse, db.DeleteUpstream(u2))
	assert.Nil(t, db.DeleteRoute(r))
	assert.Nil(t, db.DeleteUpstream(u))
	assert.Nil(t, db.DeleteStreamRoute(sr))
	assert.Nil(t, db.DeleteUpstream(u2))
}

func TestMemDBCacheStreamRoute(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	r1 := &v1.StreamRoute{
		ID: "1",
	}
	assert.Nil(t, c.InsertStreamRoute(r1), "inserting stream route 1")

	r, err := c.GetStreamRoute("1")
	assert.Nil(t, err)
	assert.Equal(t, r1, r)

	r2 := &v1.StreamRoute{
		ID: "2",
	}
	r3 := &v1.StreamRoute{
		ID: "3",
	}
	assert.Nil(t, c.InsertStreamRoute(r2), "inserting stream route r2")
	assert.Nil(t, c.InsertStreamRoute(r3), "inserting stream route r3")

	r, err = c.GetStreamRoute("3")
	assert.Nil(t, err)
	assert.Equal(t, r3, r)

	assert.Nil(t, c.DeleteStreamRoute(r3), "delete stream route r3")

	routes, err := c.ListStreamRoutes()
	assert.Nil(t, err, "listing streams routes")

	if routes[0].ID > routes[1].ID {
		routes[0], routes[1] = routes[1], routes[0]
	}
	assert.Equal(t, routes[0], r1)
	assert.Equal(t, routes[1], r2)

	r4 := &v1.StreamRoute{
		ID: "4",
	}
	assert.Error(t, ErrNotFound, c.DeleteStreamRoute(r4))
}

func TestMemDBCacheGlobalRule(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	gr1 := &v1.GlobalRule{
		ID: "1",
	}
	assert.Nil(t, c.InsertGlobalRule(gr1), "inserting global rule 1")

	gr, err := c.GetGlobalRule("1")
	assert.Nil(t, err)
	assert.Equal(t, gr1, gr)

	gr2 := &v1.GlobalRule{
		ID: "2",
	}
	gr3 := &v1.GlobalRule{
		ID: "3",
	}
	assert.Nil(t, c.InsertGlobalRule(gr2), "inserting global_rule r2")
	assert.Nil(t, c.InsertGlobalRule(gr3), "inserting global_rule r3")

	gr, err = c.GetGlobalRule("3")
	assert.Nil(t, err)
	assert.Equal(t, gr, gr3)

	assert.Nil(t, c.DeleteGlobalRule(gr), "delete global_rule r3")

	grs, err := c.ListGlobalRules()
	assert.Nil(t, err, "listing global rules")

	if grs[0].ID > grs[1].ID {
		grs[0], grs[1] = grs[1], grs[0]
	}
	assert.Equal(t, grs[0], gr1)
	assert.Equal(t, grs[1], gr2)

	gr4 := &v1.GlobalRule{
		ID: "4",
	}
	assert.Error(t, ErrNotFound, c.DeleteGlobalRule(gr4))
}

func TestMemDBCacheConsumer(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	c1 := &v1.Consumer{
		Username: "jack",
	}
	assert.Nil(t, c.InsertConsumer(c1), "inserting consumer c1")

	c11, err := c.GetConsumer("jack")
	assert.Nil(t, err)
	assert.Equal(t, c1, c11)

	c2 := &v1.Consumer{
		Username: "tom",
	}
	c3 := &v1.Consumer{
		Username: "jerry",
	}
	assert.Nil(t, c.InsertConsumer(c2), "inserting consumer c2")
	assert.Nil(t, c.InsertConsumer(c3), "inserting consumer c3")

	c22, err := c.GetConsumer("tom")
	assert.Nil(t, err)
	assert.Equal(t, c2, c22)

	assert.Nil(t, c.DeleteConsumer(c3), "delete consumer c3")

	consumers, err := c.ListConsumers()
	assert.Nil(t, err, "listing consumers")

	if consumers[0].Username > consumers[1].Username {
		consumers[0], consumers[1] = consumers[1], consumers[0]
	}
	assert.Equal(t, consumers[0], c1)
	assert.Equal(t, consumers[1], c2)

	c4 := &v1.Consumer{
		Username: "chandler",
	}
	assert.Error(t, ErrNotFound, c.DeleteConsumer(c4))
}

func TestMemDBCacheSchema(t *testing.T) {
	c, err := NewMemDBCache()
	assert.Nil(t, err, "NewMemDBCache")

	s1 := &v1.Schema{
		Name:    "plugins/p1",
		Content: "plugin schema",
	}
	assert.Nil(t, c.InsertSchema(s1), "inserting schema s1")

	s11, err := c.GetSchema("plugins/p1")
	assert.Nil(t, err)
	assert.Equal(t, s1, s11)

	s2 := &v1.Schema{
		Name: "plugins/p2",
	}
	s3 := &v1.Schema{
		Name: "plugins/p3",
	}
	assert.Nil(t, c.InsertSchema(s2), "inserting schema s2")
	assert.Nil(t, c.InsertSchema(s3), "inserting schema s3")

	s22, err := c.GetSchema("plugins/p2")
	assert.Nil(t, err)
	assert.Equal(t, s2, s22)

	assert.Nil(t, c.DeleteSchema(s3), "delete schema s3")

	schemaList, err := c.ListSchema()
	assert.Nil(t, err, "listing schema")

	if schemaList[0].Name > schemaList[1].Name {
		schemaList[0], schemaList[1] = schemaList[1], schemaList[0]
	}
	assert.Equal(t, schemaList[0], s1)
	assert.Equal(t, schemaList[1], s2)

	s4 := &v1.Schema{
		Name: "plugins/p4",
	}
	assert.Error(t, ErrNotFound, c.DeleteSchema(s4))
}
