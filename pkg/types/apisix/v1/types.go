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
package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	// HashOnVars means the hash scope is variable.
	HashOnVars = "vars"
	// HashOnVarsCombination means the hash scope is the
	// variable combination.
	HashOnVarsCombination = "vars_combinations"
	// HashOnHeader means the hash scope is HTTP request
	// headers.
	HashOnHeader = "header"
	// HashOnCookie means the hash scope is HTTP Cookie.
	HashOnCookie = "cookie"
	// HashOnConsumer means the hash scope is APISIX consumer.
	HashOnConsumer = "consumer"

	// LbRoundRobin is the round robin load balancer.
	LbRoundRobin = "roundrobin"
	// LbConsistentHash is the consistent hash load balancer.
	LbConsistentHash = "chash"
	// LbEwma is the ewma load balancer.
	LbEwma = "ewma"
	// LbLeaseConn is the least connection load balancer.
	LbLeastConn = "least_conn"

	// SchemeHTTP represents the HTTP protocol.
	SchemeHTTP = "http"
	// SchemeGRPC represents the GRPC protocol.
	SchemeGRPC = "grpc"

	// HealthCheckHTTP represents the HTTP kind health check.
	HealthCheckHTTP = "http"
	// HealthCheckHTTPS represents the HTTPS kind health check.
	HealthCheckHTTPS = "https"
	// HealthCheckTCP represents the TCP kind health check.
	HealthCheckTCP = "tcp"

	// HealthCheckMaxConsecutiveNumber is the max number for
	// the consecutive success/failure in upstream health check.
	HealthCheckMaxConsecutiveNumber = 254
	// ActiveHealthCheckMinInterval is the minimum interval for
	// the active health check.
	ActiveHealthCheckMinInterval = time.Second

	// DefaultUpstreamTimeout represents the default connect,
	// read and send timeout (in seconds) with upstreams.
	DefaultUpstreamTimeout = 60
)

// Metadata contains all meta information about resources.
// +k8s:deepcopy-gen=true
type Metadata struct {
	ID     string            `json:"id,omitempty" yaml:"id,omitempty"`
	Name   string            `json:"name,omitempty" yaml:"name,omitempty"`
	Desc   string            `json:"desc,omitempty" yaml:"desc,omitempty"`
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// Route apisix route object
// +k8s:deepcopy-gen=true
type Route struct {
	Metadata `json:",inline" yaml:",inline"`

	Host            string   `json:"host,omitempty" yaml:"host,omitempty"`
	Hosts           []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Uri             string   `json:"uri,omitempty" yaml:"uri,omitempty"`
	Priority        int      `json:"priority,omitempty" yaml:"priority,omitempty"`
	Vars            Vars     `json:"vars,omitempty" yaml:"vars,omitempty"`
	Uris            []string `json:"uris,omitempty" yaml:"uris,omitempty"`
	Methods         []string `json:"methods,omitempty" yaml:"methods,omitempty"`
	EnableWebsocket bool     `json:"enable_websocket,omitempty" yaml:"enable_websocket,omitempty"`
	RemoteAddrs     []string `json:"remote_addrs,omitempty" yaml:"remote_addrs,omitempty"`
	UpstreamId      string   `json:"upstream_id,omitempty" yaml:"upstream_id,omitempty"`
	Plugins         Plugins  `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// Vars represents the route match expressions of APISIX.
type Vars [][]StringOrSlice

// UnmarshalJSON implements json.Unmarshaler interface.
// lua-cjson doesn't distinguish empty array and table,
// and by default empty array will be encoded as '{}'.
// We have to maintain the compatibility.
func (vars *Vars) UnmarshalJSON(p []byte) error {
	if p[0] == '{' {
		if len(p) != 2 {
			return errors.New("unexpected non-empty object")
		}
		return nil
	}
	var data [][]StringOrSlice
	if err := json.Unmarshal(p, &data); err != nil {
		return err
	}
	*vars = data
	return nil
}

// StringOrSlice represents a string or a string slice.
// TODO Do not use interface{} to avoid the reflection overheads.
// +k8s:deepcopy-gen=true
type StringOrSlice struct {
	StrVal   string   `json:"-"`
	SliceVal []string `json:"-"`
}

func (s *StringOrSlice) MarshalJSON() ([]byte, error) {
	var (
		p   []byte
		err error
	)
	if s.SliceVal != nil {
		p, err = json.Marshal(s.SliceVal)
	} else {
		p, err = json.Marshal(s.StrVal)
	}
	return p, err
}

func (s *StringOrSlice) UnmarshalJSON(p []byte) error {
	var err error

	if len(p) == 0 {
		return errors.New("empty object")
	}
	if p[0] == '[' {
		err = json.Unmarshal(p, &s.SliceVal)
	} else {
		err = json.Unmarshal(p, &s.StrVal)
	}
	return err
}

type Plugins map[string]interface{}

func (p *Plugins) DeepCopyInto(out *Plugins) {
	b, _ := json.Marshal(&p)
	_ = json.Unmarshal(b, out)
}

func (p *Plugins) DeepCopy() *Plugins {
	if p == nil {
		return nil
	}
	out := new(Plugins)
	p.DeepCopyInto(out)
	return out
}

// Upstream is the apisix upstream definition.
// +k8s:deepcopy-gen=true
type Upstream struct {
	Metadata `json:",inline" yaml:",inline"`

	Type    string               `json:"type,omitempty" yaml:"type,omitempty"`
	HashOn  string               `json:"hash_on,omitempty" yaml:"hash_on,omitempty"`
	Key     string               `json:"key,omitempty" yaml:"key,omitempty"`
	Checks  *UpstreamHealthCheck `json:"checks,omitempty" yaml:"checks,omitempty"`
	Nodes   UpstreamNodes        `json:"nodes" yaml:"nodes"`
	Scheme  string               `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	Retries int                  `json:"retries,omitempty" yaml:"retries,omitempty"`
	Timeout *UpstreamTimeout     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// UpstreamTimeout represents the timeout settings on Upstream.
type UpstreamTimeout struct {
	// Connect is the connect timeout
	Connect int `json:"connect" yaml:"connect"`
	// Send is the send timeout
	Send int `json:"send" yaml:"send"`
	// Read is the read timeout
	Read int `json:"read" yaml:"read"`
}

// UpstreamNodes is the upstream node list.
type UpstreamNodes []UpstreamNode

// UnmarshalJSON implements json.Unmarshaler interface.
// lua-cjson doesn't distinguish empty array and table,
// and by default empty array will be encoded as '{}'.
// We have to maintain the compatibility.
func (n *UpstreamNodes) UnmarshalJSON(p []byte) error {
	if p[0] == '{' {
		if len(p) != 2 {
			return errors.New("unexpected non-empty object")
		}
		return nil
	}
	var data []UpstreamNode
	if err := json.Unmarshal(p, &data); err != nil {
		return err
	}
	*n = data
	return nil
}

// UpstreamNode is the node in upstream
// +k8s:deepcopy-gen=true
type UpstreamNode struct {
	Host   string `json:"host,omitempty" yaml:"host,omitempty"`
	Port   int    `json:"port,omitempty" yaml:"port,omitempty"`
	Weight int    `json:"weight,omitempty" yaml:"weight,omitempty"`
}

// UpstreamHealthCheck defines the active and/or passive health check for an Upstream,
// with the upstream health check feature, pods can be kicked out or joined in quickly,
// if the feedback of Kubernetes liveness/readiness probe is long.
// +k8s:deepcopy-gen=true
type UpstreamHealthCheck struct {
	Active  *UpstreamActiveHealthCheck  `json:"active" yaml:"active"`
	Passive *UpstreamPassiveHealthCheck `json:"passive,omitempty" yaml:"passive,omitempty"`
}

// UpstreamActiveHealthCheck defines the active kind of upstream health check.
// +k8s:deepcopy-gen=true
type UpstreamActiveHealthCheck struct {
	Type               string                             `json:"type,omitempty" yaml:"type,omitempty"`
	Timeout            int                                `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Concurrency        int                                `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	Host               string                             `json:"host,omitempty" yaml:"host,omitempty"`
	Port               int32                              `json:"port,omitempty" yaml:"port,omitempty"`
	HTTPPath           string                             `json:"http_path,omitempty" yaml:"http_path,omitempty"`
	HTTPSVerifyCert    bool                               `json:"https_verify_certificate,omitempty" yaml:"https_verify_certificate,omitempty"`
	HTTPRequestHeaders []string                           `json:"req_headers,omitempty" yaml:"req_headers,omitempty"`
	Healthy            UpstreamActiveHealthCheckHealthy   `json:"healthy,omitempty" yaml:"healthy,omitempty"`
	Unhealthy          UpstreamActiveHealthCheckUnhealthy `json:"unhealthy,omitempty" yaml:"unhealthy,omitempty"`
}

// UpstreamPassiveHealthCheck defines the passive kind of upstream health check.
// +k8s:deepcopy-gen=true
type UpstreamPassiveHealthCheck struct {
	Type      string                              `json:"type,omitempty" yaml:"type,omitempty"`
	Healthy   UpstreamPassiveHealthCheckHealthy   `json:"healthy,omitempty" yaml:"healthy,omitempty"`
	Unhealthy UpstreamPassiveHealthCheckUnhealthy `json:"unhealthy,omitempty" yaml:"unhealthy,omitempty"`
}

// UpstreamActiveHealthCheckHealthy defines the conditions to judge whether
// an upstream node is healthy with the active manner.
// +k8s:deepcopy-gen=true
type UpstreamActiveHealthCheckHealthy struct {
	UpstreamPassiveHealthCheckHealthy `json:",inline" yaml:",inline"`

	Interval int `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// UpstreamPassiveHealthCheckHealthy defines the conditions to judge whether
// an upstream node is healthy with the passive manner.
// +k8s:deepcopy-gen=true
type UpstreamPassiveHealthCheckHealthy struct {
	HTTPStatuses []int `json:"http_statuses,omitempty" yaml:"http_statuses,omitempty"`
	Successes    int   `json:"successes,omitempty" yaml:"successes,omitempty"`
}

// UpstreamActiveHealthCheckUnhealthy defines the conditions to judge whether
// an upstream node is unhealthy with the active manager.
// +k8s:deepcopy-gen=true
type UpstreamActiveHealthCheckUnhealthy struct {
	UpstreamPassiveHealthCheckUnhealthy `json:",inline" yaml:",inline"`

	Interval int `json:"interval,omitempty" yaml:"interval,omitempty"`
}

// UpstreamPassiveHealthCheckUnhealthy defines the conditions to judge whether
// an upstream node is unhealthy with the passive manager.
// +k8s:deepcopy-gen=true
type UpstreamPassiveHealthCheckUnhealthy struct {
	HTTPStatuses []int   `json:"http_statuses,omitempty" yaml:"http_statuses,omitempty"`
	HTTPFailures int     `json:"http_failures,omitempty" yaml:"http_failures,omitempty"`
	TCPFailures  int     `json:"tcp_failures,omitempty" yaml:"tcp_failures,omitempty"`
	Timeouts     float64 `json:"timeouts,omitempty" yaml:"timeouts,omitempty"`
}

// Ssl apisix ssl object
// +k8s:deepcopy-gen=true
type Ssl struct {
	ID     string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Snis   []string               `json:"snis,omitempty" yaml:"snis,omitempty"`
	Cert   string                 `json:"cert,omitempty" yaml:"cert,omitempty"`
	Key    string                 `json:"key,omitempty" yaml:"key,omitempty"`
	Status int                    `json:"status,omitempty" yaml:"status,omitempty"`
	Labels map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Client *MutualTLSClientConfig `json:"client,omitempty" yaml:"client,omitempty"`
}

// MutualTLSClientConfig apisix SSL client field
// +k8s:deepcopy-gen=true
type MutualTLSClientConfig struct {
	CA    string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Depth int    `json:"depth,omitempty" yaml:"depth,omitempty"`
}

// StreamRoute represents the stream_route object in APISIX.
// +k8s:deepcopy-gen=true
type StreamRoute struct {
	// TODO metadata should use Metadata type
	ID         string            `json:"id,omitempty" yaml:"id,omitempty"`
	Desc       string            `json:"desc,omitempty" yaml:"desc,omitempty"`
	Labels     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	ServerPort int32             `json:"server_port,omitempty" yaml:"server_port,omitempty"`
	UpstreamId string            `json:"upstream_id,omitempty" yaml:"upstream_id,omitempty"`
	Upstream   *Upstream         `json:"upstream,omitempty" yaml:"upstream,omitempty"`
}

// GlobalRule represents the global_rule object in APISIX.
// +k8s:deepcopy-gen=true
type GlobalRule struct {
	ID      string  `json:"id,omitempty" yaml:"id,omitempty"`
	Plugins Plugins `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// Consumer represents the consumer object in APISIX.
// +k8s:deepcopy-gen=true
type Consumer struct {
	Username string            `json:"username" yaml:"username"`
	Desc     string            `json:"desc,omitempty" yaml:"desc,omitempty"`
	Labels   map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Plugins  Plugins           `json:"plugins,omitempty" yaml:"plugins,omitempty"`
}

// NewDefaultUpstream returns an empty Upstream with default values.
func NewDefaultUpstream() *Upstream {
	return &Upstream{
		Type:   LbRoundRobin,
		Key:    "",
		Nodes:  make(UpstreamNodes, 0),
		Scheme: SchemeHTTP,
		Metadata: Metadata{
			Desc: "Created by apisix-ingress-controller, DO NOT modify it manually",
			Labels: map[string]string{
				"managed-by": "apisix-ingress-controller",
			},
		},
	}
}

// NewDefaultRoute returns an empty Route with default values.
func NewDefaultRoute() *Route {
	return &Route{
		Metadata: Metadata{
			Desc: "Created by apisix-ingress-controller, DO NOT modify it manually",
			Labels: map[string]string{
				"managed-by": "apisix-ingress-controller",
			},
		},
	}
}

// NewDefaultStreamRoute returns an empty StreamRoute with default values.
func NewDefaultStreamRoute() *StreamRoute {
	return &StreamRoute{
		Desc: "Created by apisix-ingress-controller, DO NOT modify it manually",
		Labels: map[string]string{
			"managed-by": "apisix-ingress-controller",
		},
	}
}

// NewDefaultConsumer returns an empty Consumer with default values.
func NewDefaultConsumer() *Consumer {
	return &Consumer{
		Desc: "Created by apisix-ingress-controller, DO NOT modify it manually",
		Labels: map[string]string{
			"managed-by": "apisix-ingress-controller",
		},
	}
}

// ComposeUpstreamName uses namespace, name, subset (optional) and port info to compose
// the upstream name.
func ComposeUpstreamName(namespace, name, subset string, port int32) string {
	pstr := strconv.Itoa(int(port))
	// FIXME Use sync.Pool to reuse this buffer if the upstream
	// name composing code path is hot.
	var p []byte
	if subset == "" {
		p = make([]byte, 0, len(namespace)+len(name)+len(pstr)+2)
	} else {
		p = make([]byte, 0, len(namespace)+len(name)+len(subset)+len(pstr)+3)
	}

	buf := bytes.NewBuffer(p)
	buf.WriteString(namespace)
	buf.WriteByte('_')
	buf.WriteString(name)
	buf.WriteByte('_')
	if subset != "" {
		buf.WriteString(subset)
		buf.WriteByte('_')
	}
	buf.WriteString(pstr)

	return buf.String()
}

// ComposeRouteName uses namespace, name and rule name to compose
// the route name.
func ComposeRouteName(namespace, name string, rule string) string {
	// FIXME Use sync.Pool to reuse this buffer if the upstream
	// name composing code path is hot.
	p := make([]byte, 0, len(namespace)+len(name)+len(rule)+2)
	buf := bytes.NewBuffer(p)

	buf.WriteString(namespace)
	buf.WriteByte('_')
	buf.WriteString(name)
	buf.WriteByte('_')
	buf.WriteString(rule)

	return buf.String()
}

// ComposeStreamRouteName uses namespace, name and rule name to compose
// the stream_route name.
func ComposeStreamRouteName(namespace, name string, rule string) string {
	// FIXME Use sync.Pool to reuse this buffer if the upstream
	// name composing code path is hot.
	p := make([]byte, 0, len(namespace)+len(name)+len(rule)+6)
	buf := bytes.NewBuffer(p)

	buf.WriteString(namespace)
	buf.WriteByte('_')
	buf.WriteString(name)
	buf.WriteByte('_')
	buf.WriteString(rule)
	buf.WriteString("_tcp")

	return buf.String()
}

// ComposeConsumerName uses namespace and name of ApisixConsumer to compose
// the Consumer name.
func ComposeConsumerName(namespace, name string) string {
	p := make([]byte, 0, len(namespace)+len(name)+1)
	buf := bytes.NewBuffer(p)

	// TODO If APISIX modifies the consumer name schema, we can drop this.
	buf.WriteString(strings.Replace(namespace, "-", "_", -1))
	buf.WriteString("_")
	buf.WriteString(name)

	return buf.String()
}

// Schema represents the schema of APISIX objects.
type Schema struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Content string `json:"content,omitempty" yaml:"content,omitempty"`
}

func (s *Schema) DeepCopyInto(out *Schema) {
	b, _ := json.Marshal(&s)
	_ = json.Unmarshal(b, out)
}

func (s *Schema) DeepCopy() *Schema {
	if s == nil {
		return nil
	}
	out := new(Schema)
	s.DeepCopyInto(out)
	return out
}
