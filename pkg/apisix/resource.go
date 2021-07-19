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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/apisix-ingress-controller/pkg/log"
	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

type getResponse struct {
	Item item `json:"node"`
}

// listResponse is the unified LIST response mapping of APISIX.
type listResponse struct {
	Count IntOrString `json:"count"`
	Node  node        `json:"node"`
}

// IntOrString processing number and string types, after json deserialization will output int
type IntOrString struct {
	IntValue int `json:"int_value"`
}

func (ios *IntOrString) UnmarshalJSON(p []byte) error {
	result := strings.Trim(string(p), "\"")
	count, err := strconv.Atoi(result)
	if err != nil {
		return err
	}
	ios.IntValue = count
	return nil
}

type createResponse struct {
	Action string `json:"action"`
	Item   item   `json:"node"`
}

type updateResponse = createResponse

type node struct {
	Key   string `json:"key"`
	Items items  `json:"nodes"`
}

type items []item

// UnmarshalJSON implements json.Unmarshaler interface.
// lua-cjson doesn't distinguish empty array and table,
// and by default empty array will be encoded as '{}'.
// We have to maintain the compatibility.
func (items *items) UnmarshalJSON(p []byte) error {
	if p[0] == '{' {
		if len(p) != 2 {
			return errors.New("unexpected non-empty object")
		}
		return nil
	}
	var data []item
	if err := json.Unmarshal(p, &data); err != nil {
		return err
	}
	*items = data
	return nil
}

type item struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// route decodes item.Value and converts it to v1.Route.
func (i *item) route() (*v1.Route, error) {
	log.Debugf("got route: %s", string(i.Value))
	list := strings.Split(i.Key, "/")
	if len(list) < 1 {
		return nil, fmt.Errorf("bad route config key: %s", i.Key)
	}

	var route v1.Route
	if err := json.Unmarshal(i.Value, &route); err != nil {
		return nil, err
	}
	return &route, nil
}

// streamRoute decodes item.Value and converts it to v1.StreamRoute.
func (i *item) streamRoute() (*v1.StreamRoute, error) {
	log.Debugf("got stream_route: %s", string(i.Value))
	list := strings.Split(i.Key, "/")
	if len(list) < 1 {
		return nil, fmt.Errorf("bad stream_route config key: %s", i.Key)
	}

	var streamRoute v1.StreamRoute
	if err := json.Unmarshal(i.Value, &streamRoute); err != nil {
		return nil, err
	}
	return &streamRoute, nil
}

// upstream decodes item.Value and converts it to v1.Upstream.
func (i *item) upstream() (*v1.Upstream, error) {
	log.Debugf("got upstream: %s", string(i.Value))
	list := strings.Split(i.Key, "/")
	if len(list) < 1 {
		return nil, fmt.Errorf("bad upstream config key: %s", i.Key)
	}

	var ups v1.Upstream
	if err := json.Unmarshal(i.Value, &ups); err != nil {
		return nil, err
	}

	// This is a work around scheme to avoid APISIX's
	// health check schema about the health checker intervals.
	if ups.Checks != nil && ups.Checks.Active != nil {
		if ups.Checks.Active.Healthy.Interval == 0 {
			ups.Checks.Active.Healthy.Interval = int(v1.ActiveHealthCheckMinInterval.Seconds())
		}
		if ups.Checks.Active.Unhealthy.Interval == 0 {
			ups.Checks.Active.Healthy.Interval = int(v1.ActiveHealthCheckMinInterval.Seconds())
		}
	}
	return &ups, nil
}

// ssl decodes item.Value and converts it to v1.Ssl.
func (i *item) ssl() (*v1.Ssl, error) {
	log.Debugf("got ssl: %s", string(i.Value))
	var ssl v1.Ssl
	if err := json.Unmarshal(i.Value, &ssl); err != nil {
		return nil, err
	}
	return &ssl, nil
}

// globalRule decodes item.Value and converts it to v1.GlobalRule.
func (i *item) globalRule() (*v1.GlobalRule, error) {
	log.Debugf("got global_rule: %s", string(i.Value))
	var globalRule v1.GlobalRule
	if err := json.Unmarshal(i.Value, &globalRule); err != nil {
		return nil, err
	}
	return &globalRule, nil
}

// consumer decodes item.Value and converts it to v1.Consumer.
func (i *item) consumer() (*v1.Consumer, error) {
	log.Debugf("got consumer: %s", string(i.Value))
	var consumer v1.Consumer
	if err := json.Unmarshal(i.Value, &consumer); err != nil {
		return nil, err
	}
	return &consumer, nil
}
