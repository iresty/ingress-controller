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
package ingress

import (
	"context"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/apache/apisix-ingress-controller/pkg/apisix"
	apisixcache "github.com/apache/apisix-ingress-controller/pkg/apisix/cache"
	configv1 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v1"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	"github.com/apache/apisix-ingress-controller/pkg/types"
	apisixv1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

type endpointsController struct {
	controller *Controller
	workqueue  workqueue.RateLimitingInterface
	workers    int
}

func (c *Controller) newEndpointsController() *endpointsController {
	ctl := &endpointsController{
		controller: c,
		workqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(1*time.Second, 60*time.Second, 5), "endpoints"),
		workers:    1,
	}

	ctl.controller.epInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ctl.onAdd,
			UpdateFunc: ctl.onUpdate,
			DeleteFunc: ctl.onDelete,
		},
	)

	return ctl
}

func (c *endpointsController) run(ctx context.Context) {
	log.Info("endpoints controller started")
	defer log.Info("endpoints controller exited")
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(ctx.Done(), c.controller.epInformer.HasSynced); !ok {
		log.Error("informers sync failed")
		return
	}

	handler := func() {
		for {
			obj, shutdown := c.workqueue.Get()
			if shutdown {
				return
			}

			err := c.sync(ctx, obj.(*types.Event))
			c.workqueue.Done(obj)
			c.handleSyncErr(obj, err)
		}
	}

	for i := 0; i < c.workers; i++ {
		go handler()
	}

	<-ctx.Done()
}

func (c *endpointsController) sync(ctx context.Context, ev *types.Event) error {
	ep := ev.Object.(*corev1.Endpoints)
	svc, err := c.controller.svcLister.Services(ep.Namespace).Get(ep.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Infof("service %s/%s not found", ep.Namespace, ep.Name)
			return nil
		}
		log.Errorf("failed to get service %s/%s: %s", ep.Namespace, ep.Name, err)
		return err
	}
	var subsets []configv1.ApisixUpstreamSubset
	subsets = append(subsets, configv1.ApisixUpstreamSubset{})
	au, err := c.controller.apisixUpstreamLister.ApisixUpstreams(ep.Namespace).Get(ep.Name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Errorf("failed to get ApisixUpstream %s/%s: %s", ep.Namespace, ep.Name, err)
			return err
		}
	} else if len(au.Spec.Subsets) > 0 {
		subsets = append(subsets, au.Spec.Subsets...)
	}

	portMap := make(map[string]int32)
	for _, port := range svc.Spec.Ports {
		portMap[port.Name] = port.Port
	}
	clusters := c.controller.apisix.ListClusters()

	//endpoints turn to zero
	if len(ep.Subsets) == 0 {
		nodes := []apisixv1.UpstreamNode{}
		for _, port := range svc.Spec.Ports {
			svcPort := port.Port
			name := apisixv1.ComposeUpstreamName(ep.Namespace, ep.Name, "", svcPort)
			for _, cluster := range clusters {
				if err := c.syncToCluster(ctx, cluster, nodes, name); err != nil {
					return err
				}
			}
		}
	}

	for _, s := range ep.Subsets {
		for _, port := range s.Ports {
			svcPort, ok := portMap[port.Name]
			if !ok {
				// This shouldn't happen.
				log.Errorf("port %s in endpoints %s/%s but not in service", port.Name, ep.Namespace, ep.Name)
				continue
			}
			for _, subset := range subsets {
				nodes, err := c.controller.translator.TranslateUpstreamNodes(ep, svcPort, subset.Labels)
				if err != nil {
					log.Errorw("failed to translate upstream nodes",
						zap.Error(err),
						zap.Any("endpoints", ep),
						zap.Int32("port", svcPort),
					)
				}
				name := apisixv1.ComposeUpstreamName(ep.Namespace, ep.Name, subset.Name, svcPort)
				for _, cluster := range clusters {
					if err := c.syncToCluster(ctx, cluster, nodes, name); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (c *endpointsController) syncToCluster(ctx context.Context, cluster apisix.Cluster, nodes apisixv1.UpstreamNodes, upsName string) error {
	upstream, err := cluster.Upstream().Get(ctx, upsName)
	if err != nil {
		if err == apisixcache.ErrNotFound {
			log.Warnw("upstream is not referenced",
				zap.String("cluster", cluster.String()),
				zap.String("upstream", upsName),
			)
			return nil
		} else {
			log.Errorw("failed to get upstream",
				zap.String("upstream", upsName),
				zap.String("cluster", cluster.String()),
				zap.Error(err),
			)
			return err
		}
	}

	upstream.Nodes = nodes

	log.Debugw("upstream binds new nodes",
		zap.Any("upstream", upstream),
		zap.String("cluster", cluster.String()),
	)

	updated := &manifest{
		upstreams: []*apisixv1.Upstream{upstream},
	}
	return c.controller.syncManifests(ctx, nil, updated, nil)
}

func (c *endpointsController) handleSyncErr(obj interface{}, err error) {
	if err == nil {
		c.workqueue.Forget(obj)
		return
	}
	log.Warnw("sync endpoints failed, will retry",
		zap.Any("object", obj),
	)
	c.workqueue.AddRateLimited(obj)
}

func (c *endpointsController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found endpoints object with bad namespace/name: %s, ignore it", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	log.Debugw("endpoints add event arrived",
		zap.String("object-key", key))

	c.workqueue.AddRateLimited(&types.Event{
		Type:   types.EventAdd,
		Object: obj,
	})
}

func (c *endpointsController) onUpdate(prev, curr interface{}) {
	prevEp := prev.(*corev1.Endpoints)
	currEp := curr.(*corev1.Endpoints)

	if prevEp.GetResourceVersion() == currEp.GetResourceVersion() {
		return
	}
	key, err := cache.MetaNamespaceKeyFunc(currEp)
	if err != nil {
		log.Errorf("found endpoints object with bad namespace/name: %s, ignore it", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	log.Debugw("endpoints update event arrived",
		zap.Any("new object", currEp),
		zap.Any("old object", prevEp),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type:   types.EventUpdate,
		Object: curr,
	})
}

func (c *endpointsController) onDelete(obj interface{}) {
	ep, ok := obj.(*corev1.Endpoints)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			log.Errorf("found endpoints: %+v in bad tombstone state", obj)
			return
		}
		ep = tombstone.Obj.(*corev1.Endpoints)
	}

	// FIXME Refactor Controller.namespaceWatching to just use
	// namespace after all controllers use the same way to fetch
	// the object.
	if !c.controller.namespaceWatching(ep.Namespace + "/" + ep.Name) {
		return
	}
	log.Debugw("endpoints delete event arrived",
		zap.Any("final state", ep),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type:   types.EventDelete,
		Object: ep,
	})
}
