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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/apache/apisix-ingress-controller/pkg/apisix/cache"
	"github.com/apache/apisix-ingress-controller/pkg/log"
)

const (
	_defaultTimeout = 5 * time.Second

	_cacheSyncing = iota
	_cacheSynced
)

var (
	// ErrClusterNotExist means a cluster doesn't exist.
	ErrClusterNotExist = errors.New("client not exist")
	// ErrDuplicatedCluster means the cluster adding request was
	// rejected since the cluster was already created.
	ErrDuplicatedCluster = errors.New("duplicated cluster")

	_errReadOnClosedResBody = errors.New("http: read on closed response body")

	// Default shared transport for apisix client
	_defaultTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).Dial,
		DialContext: (&net.Dialer{
			Timeout: 3 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
)

// ClusterOptions contains parameters to customize APISIX client.
type ClusterOptions struct {
	Name     string
	AdminKey string
	BaseURL  string
	Timeout  time.Duration
}

type cluster struct {
	name         string
	baseURL      string
	baseURLHost  string
	adminKey     string
	cli          *http.Client
	cacheState   int32
	cache        cache.Cache
	cacheSynced  chan struct{}
	cacheSyncErr error
	route        Route
	upstream     Upstream
	ssl          SSL
	streamRoute  StreamRoute
	globalRules  GlobalRule
	consumer     Consumer
	plugin       Plugin
}

func newCluster(o *ClusterOptions) (Cluster, error) {
	if o.BaseURL == "" {
		return nil, errors.New("empty base url")
	}
	if o.Timeout == time.Duration(0) {
		o.Timeout = _defaultTimeout
	}
	o.BaseURL = strings.TrimSuffix(o.BaseURL, "/")

	u, err := url.Parse(o.BaseURL)
	if err != nil {
		return nil, err
	}

	c := &cluster{
		name:        o.Name,
		baseURL:     o.BaseURL,
		baseURLHost: u.Host,
		adminKey:    o.AdminKey,
		cli: &http.Client{
			Timeout:   o.Timeout,
			Transport: _defaultTransport,
		},
		cacheState:  _cacheSyncing, // default state
		cacheSynced: make(chan struct{}),
	}
	c.route = newRouteClient(c)
	c.upstream = newUpstreamClient(c)
	c.ssl = newSSLClient(c)
	c.streamRoute = newStreamRouteClient(c)
	c.globalRules = newGlobalRuleClient(c)
	c.consumer = newConsumerClient(c)
	c.plugin = newPluginClient(c)

	c.cache, err = cache.NewMemDBCache()
	if err != nil {
		return nil, err
	}

	go c.syncCache()

	return c, nil
}

func (c *cluster) syncCache() {
	log.Infow("syncing cache", zap.String("cluster", c.name))
	now := time.Now()
	defer func() {
		if c.cacheSyncErr == nil {
			log.Infow("cache synced",
				zap.String("cost_time", time.Since(now).String()),
				zap.String("cluster", c.name),
			)
		} else {
			log.Errorw("failed to sync cache",
				zap.String("cost_time", time.Since(now).String()),
				zap.String("cluster", c.name),
			)
		}
	}()

	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   1,
		Steps:    5,
	}
	var lastSyncErr error
	err := wait.ExponentialBackoff(backoff, func() (done bool, _ error) {
		// impossibly return: false, nil
		// so can safe used
		done, lastSyncErr = c.syncCacheOnce()
		return
	})
	if err != nil {
		// if ErrWaitTimeout then set lastSyncErr
		c.cacheSyncErr = lastSyncErr
	}
	close(c.cacheSynced)

	if !atomic.CompareAndSwapInt32(&c.cacheState, _cacheSyncing, _cacheSynced) {
		panic("dubious state when sync cache")
	}
}

func (c *cluster) syncCacheOnce() (bool, error) {
	routes, err := c.route.List(context.TODO())
	if err != nil {
		log.Errorf("failed to list route in APISIX: %s", err)
		return false, err
	}
	upstreams, err := c.upstream.List(context.TODO())
	if err != nil {
		log.Errorf("failed to list upstreams in APISIX: %s", err)
		return false, err
	}
	ssl, err := c.ssl.List(context.TODO())
	if err != nil {
		log.Errorf("failed to list ssl in APISIX: %s", err)
		return false, err
	}
	streamRoutes, err := c.streamRoute.List(context.TODO())
	if err != nil {
		log.Errorf("failed to list stream_routes in APISIX: %s", err)
		return false, err
	}
	globalRules, err := c.globalRules.List(context.TODO())
	if err != nil {
		log.Errorf("failed to list global_rules in APISIX: %s", err)
		return false, err
	}
	consumers, err := c.consumer.List(context.TODO())
	if err != nil {
		log.Errorf("failed to list consumers in APISIX: %s", err)
		return false, err
	}

	for _, r := range routes {
		if err := c.cache.InsertRoute(r); err != nil {
			log.Errorw("failed to insert route to cache",
				zap.String("route", r.ID),
				zap.String("cluster", c.name),
				zap.String("error", err.Error()),
			)
			return false, err
		}
	}
	for _, u := range upstreams {
		if err := c.cache.InsertUpstream(u); err != nil {
			log.Errorw("failed to insert upstream to cache",
				zap.String("upstream", u.ID),
				zap.String("cluster", c.name),
				zap.String("error", err.Error()),
			)
			return false, err
		}
	}
	for _, s := range ssl {
		if err := c.cache.InsertSSL(s); err != nil {
			log.Errorw("failed to insert ssl to cache",
				zap.String("ssl", s.ID),
				zap.String("cluster", c.name),
				zap.String("error", err.Error()),
			)
			return false, err
		}
	}
	for _, sr := range streamRoutes {
		if err := c.cache.InsertStreamRoute(sr); err != nil {
			log.Errorw("failed to insert stream_route to cache",
				zap.Any("stream_route", sr),
				zap.String("cluster", c.name),
				zap.String("error", err.Error()),
			)
			return false, err
		}
	}
	for _, gr := range globalRules {
		if err := c.cache.InsertGlobalRule(gr); err != nil {
			log.Errorw("failed to insert global_rule to cache",
				zap.Any("global_rule", gr),
				zap.String("cluster", c.name),
				zap.String("error", err.Error()),
			)
			return false, err
		}
	}
	for _, consumer := range consumers {
		if err := c.cache.InsertConsumer(consumer); err != nil {
			log.Errorw("failed to insert consumer to cache",
				zap.Any("consumer", consumer),
				zap.String("cluster", c.name),
				zap.String("error", err.Error()),
			)
		}
	}
	return true, nil
}

// String implements Cluster.String method.
func (c *cluster) String() string {
	return fmt.Sprintf("name=%s; base_url=%s", c.name, c.baseURL)
}

// HasSynced implements Cluster.HasSynced method.
func (c *cluster) HasSynced(ctx context.Context) error {
	if c.cacheSyncErr != nil {
		return c.cacheSyncErr
	}
	if atomic.LoadInt32(&c.cacheState) == _cacheSynced {
		return nil
	}

	// still in sync
	now := time.Now()
	log.Warnf("waiting cluster %s to ready, it may takes a while", c.name)
	select {
	case <-ctx.Done():
		log.Errorf("failed to wait cluster to ready: %s", ctx.Err())
		return ctx.Err()
	case <-c.cacheSynced:
		if c.cacheSyncErr != nil {
			// See https://github.com/apache/apisix-ingress-controller/issues/448
			// for more details.
			return c.cacheSyncErr
		}
		log.Warnf("cluster %s now is ready, cost time %s", c.name, time.Since(now).String())
		return nil
	}
}

// Route implements Cluster.Route method.
func (c *cluster) Route() Route {
	return c.route
}

// Upstream implements Cluster.Upstream method.
func (c *cluster) Upstream() Upstream {
	return c.upstream
}

// SSL implements Cluster.SSL method.
func (c *cluster) SSL() SSL {
	return c.ssl
}

// StreamRoute implements Cluster.StreamRoute method.
func (c *cluster) StreamRoute() StreamRoute {
	return c.streamRoute
}

// GlobalRule implements Cluster.GlobalRule method.
func (c *cluster) GlobalRule() GlobalRule {
	return c.globalRules
}

// Consumer implements Cluster.Consumer method.
func (c *cluster) Consumer() Consumer {
	return c.consumer
}

// Plugin implements Cluster.Plugin method.
func (c *cluster) Plugin() Plugin {
	return c.plugin
}

// HealthCheck implements Cluster.HealthCheck method.
func (c *cluster) HealthCheck(ctx context.Context) (err error) {
	if c.cacheSyncErr != nil {
		err = c.cacheSyncErr
		return
	}
	if atomic.LoadInt32(&c.cacheState) == _cacheSyncing {
		return
	}

	// Retry three times in a row, and exit if all of them fail.
	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1,
		Steps:    3,
	}
	var lastCheckErr error
	err = wait.ExponentialBackoffWithContext(ctx, backoff, func() (done bool, _ error) {
		if lastCheckErr = c.healthCheck(ctx); lastCheckErr != nil {
			log.Warnf("failed to check health for cluster %s: %s, will retry", c.name, lastCheckErr)
			return
		}
		done = true
		return
	})
	if err != nil {
		// if ErrWaitTimeout then set lastSyncErr
		c.cacheSyncErr = lastCheckErr
	}
	return err
}

func (c *cluster) healthCheck(ctx context.Context) (err error) {
	// tcp socket probe
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.baseURLHost)
	if err != nil {
		return err
	}
	if er := conn.Close(); er != nil {
		log.Warnw("failed to close tcp probe connection",
			zap.Error(err),
			zap.String("cluster", c.name),
		)
	}
	return
}

func (c *cluster) applyAuth(req *http.Request) {
	if c.adminKey != "" {
		req.Header.Set("X-API-Key", c.adminKey)
	}
}

func (c *cluster) do(req *http.Request) (*http.Response, error) {
	c.applyAuth(req)
	return c.cli.Do(req)
}

func (c *cluster) getResource(ctx context.Context, url string) (*getResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer drainBody(resp.Body, url)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, cache.ErrNotFound
		} else {
			err = multierr.Append(err, fmt.Errorf("unexpected status code %d", resp.StatusCode))
			err = multierr.Append(err, fmt.Errorf("error message: %s", readBody(resp.Body, url)))
		}
		return nil, err
	}

	var res getResponse

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *cluster) listResource(ctx context.Context, url string) (*listResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer drainBody(resp.Body, url)
	if resp.StatusCode != http.StatusOK {
		err = multierr.Append(err, fmt.Errorf("unexpected status code %d", resp.StatusCode))
		err = multierr.Append(err, fmt.Errorf("error message: %s", readBody(resp.Body, url)))
		return nil, err
	}

	var list listResponse

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&list); err != nil {
		return nil, err
	}
	return &list, nil
}

func (c *cluster) createResource(ctx context.Context, url string, body io.Reader) (*createResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	defer drainBody(resp.Body, url)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		err = multierr.Append(err, fmt.Errorf("unexpected status code %d", resp.StatusCode))
		err = multierr.Append(err, fmt.Errorf("error message: %s", readBody(resp.Body, url)))
		return nil, err
	}

	var cr createResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (c *cluster) updateResource(ctx context.Context, url string, body io.Reader) (*updateResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer drainBody(resp.Body, url)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		err = multierr.Append(err, fmt.Errorf("unexpected status code %d", resp.StatusCode))
		err = multierr.Append(err, fmt.Errorf("error message: %s", readBody(resp.Body, url)))
		return nil, err
	}
	var ur updateResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&ur); err != nil {
		return nil, err
	}
	return &ur, nil
}

func (c *cluster) deleteResource(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer drainBody(resp.Body, url)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		err = multierr.Append(err, fmt.Errorf("unexpected status code %d", resp.StatusCode))
		message := readBody(resp.Body, url)
		err = multierr.Append(err, fmt.Errorf("error message: %s", message))
		if strings.Contains(message, "still using") {
			return cache.ErrStillInUse
		}
		return err
	}
	return nil
}

// drainBody reads whole data until EOF from r, then close it.
func drainBody(r io.ReadCloser, url string) {
	_, err := io.Copy(ioutil.Discard, r)
	if err != nil {
		if err.Error() != _errReadOnClosedResBody.Error() {
			log.Warnw("failed to drain body (read)",
				zap.String("url", url),
				zap.Error(err),
			)
		}
	}

	if err := r.Close(); err != nil {
		log.Warnw("failed to drain body (close)",
			zap.String("url", url),
			zap.Error(err),
		)
	}
}

func readBody(r io.ReadCloser, url string) string {
	defer func() {
		if err := r.Close(); err != nil {
			log.Warnw("failed to close body", zap.String("url", url), zap.Error(err))
		}
	}()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		log.Warnw("failed to read body", zap.String("url", url), zap.Error(err))
		return ""
	}
	return string(data)
}

// getSchema returns the schema of APISIX object.
func (c *cluster) getSchema(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	defer drainBody(resp.Body, url)
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", cache.ErrNotFound
		} else {
			err = multierr.Append(err, fmt.Errorf("unexpected status code %d", resp.StatusCode))
			err = multierr.Append(err, fmt.Errorf("error message: %s", readBody(resp.Body, url)))
		}
		return "", err
	}

	return readBody(resp.Body, url), nil
}
