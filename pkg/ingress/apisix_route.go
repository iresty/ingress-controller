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
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/apache/apisix-ingress-controller/pkg/kube"
	"github.com/apache/apisix-ingress-controller/pkg/kube/translation"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	"github.com/apache/apisix-ingress-controller/pkg/types"
)

const (
	_apisixrouteKey = "kubernetes.io/apisixroute.class"
)

type apisixRouteController struct {
	controller *Controller
	workqueue  workqueue.RateLimitingInterface
	workers    int
}

func (c *Controller) newApisixRouteController() *apisixRouteController {
	ctl := &apisixRouteController{
		controller: c,
		workqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(1*time.Second, 60*time.Second, 5), "ApisixRoute"),
		workers:    1,
	}
	c.apisixRouteInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ctl.onAdd,
			UpdateFunc: ctl.onUpdate,
			DeleteFunc: ctl.onDelete,
		},
	)
	return ctl
}

func (c *apisixRouteController) run(ctx context.Context) {
	log.Info("ApisixRoute controller started")
	defer log.Info("ApisixRoute controller exited")
	defer c.workqueue.ShutDown()

	ok := cache.WaitForCacheSync(ctx.Done(), c.controller.apisixRouteInformer.HasSynced)
	if !ok {
		log.Error("cache sync failed")
		return
	}

	for i := 0; i < c.workers; i++ {
		go c.runWorker(ctx)
	}
	<-ctx.Done()
}

func (c *apisixRouteController) runWorker(ctx context.Context) {
	for {
		obj, quit := c.workqueue.Get()
		if quit {
			return
		}
		err := c.sync(ctx, obj.(*types.Event))
		c.workqueue.Done(obj)
		c.handleSyncErr(obj, err)
	}
}

func (c *apisixRouteController) sync(ctx context.Context, ev *types.Event) error {
	obj := ev.Object.(kube.ApisixRouteEvent)
	namespace, name, err := cache.SplitMetaNamespaceKey(obj.Key)
	if err != nil {
		log.Errorf("invalid resource key: %s", obj.Key)
		return err
	}
	var (
		ar   kube.ApisixRoute
		tctx *translation.TranslateContext
	)
	switch obj.GroupVersion {
	case kube.ApisixRouteV1:
		ar, err = c.controller.apisixRouteLister.V1(namespace, name)
	case kube.ApisixRouteV2alpha1:
		ar, err = c.controller.apisixRouteLister.V2alpha1(namespace, name)
	case kube.ApisixRouteV2beta1:
		ar, err = c.controller.apisixRouteLister.V2beta1(namespace, name)
	}
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Errorw("failed to get ApisixRoute",
				zap.String("version", obj.GroupVersion),
				zap.String("key", obj.Key),
				zap.Error(err),
			)
			return err
		}

		if ev.Type != types.EventDelete {
			log.Warnw("ApisixRoute was deleted before it can be delivered",
				zap.String("key", obj.Key),
				zap.String("version", obj.GroupVersion),
			)
			return nil
		}
	}
	if ev.Type == types.EventDelete {
		if ar != nil {
			// We still find the resource while we are processing the DELETE event,
			// that means object with same namespace and name was created, discarding
			// this stale DELETE event.
			log.Warnw("discard the stale ApisixRoute delete event since the resource still exists",
				zap.String("key", obj.Key),
			)
			return nil
		}
		ar = ev.Tombstone.(kube.ApisixRoute)
	}
	//
	switch obj.GroupVersion {
	case kube.ApisixRouteV1:
		tctx, err = c.controller.translator.TranslateRouteV1(ar.V1())
		if err != nil {
			log.Errorw("failed to translate ApisixRoute v1",
				zap.Error(err),
				zap.Any("object", ar),
			)
			return err
		}
	case kube.ApisixRouteV2alpha1:
		if ev.Type != types.EventDelete {
			tctx, err = c.controller.translator.TranslateRouteV2alpha1(ar.V2alpha1())
		} else {
			// Use TranslateRouteV2alpha1NotStrictly in EventDelete.
			// if K8S service has been removed before ApisixRoute resource, the translation about nodes
			// of upstream will be failed.
			tctx, err = c.controller.translator.TranslateRouteV2alpha1NotStrictly(ar.V2alpha1())
		}
		if err != nil {
			log.Errorw("failed to translate ApisixRoute v2alpha1",
				zap.Error(err),
				zap.Any("object", ar),
			)
			return err
		}
	case kube.ApisixRouteV2beta1:
		if ev.Type != types.EventDelete {
			tctx, err = c.controller.translator.TranslateRouteV2beta1(ar.V2beta1())
		} else {
			tctx, err = c.controller.translator.TranslateRouteV2beta1NotStrictly(ar.V2beta1())
		}
		if err != nil {
			log.Errorw("failed to translate ApisixRoute v2beta1",
				zap.Error(err),
				zap.Any("object", ar),
			)
			return err
		}
	}

	log.Debugw("translated ApisixRoute",
		zap.Any("routes", tctx.Routes),
		zap.Any("upstreams", tctx.Upstreams),
		zap.Any("apisix_route", ar),
	)

	m := &manifest{
		routes:       tctx.Routes,
		upstreams:    tctx.Upstreams,
		streamRoutes: tctx.StreamRoutes,
	}

	var (
		added   *manifest
		updated *manifest
		deleted *manifest
	)

	if ev.Type == types.EventDelete {
		deleted = m
	} else if ev.Type == types.EventAdd {
		added = m
	} else {
		var oldCtx *translation.TranslateContext
		switch obj.GroupVersion {
		case kube.ApisixRouteV1:
			oldCtx, err = c.controller.translator.TranslateRouteV1(obj.OldObject.V1())
		case kube.ApisixRouteV2alpha1:
			oldCtx, err = c.controller.translator.TranslateRouteV2alpha1(obj.OldObject.V2alpha1())
		case kube.ApisixRouteV2beta1:
			oldCtx, err = c.controller.translator.TranslateRouteV2beta1(obj.OldObject.V2beta1())
		}
		if err != nil {
			log.Errorw("failed to translate old ApisixRoute",
				zap.String("version", obj.GroupVersion),
				zap.String("event", "update"),
				zap.Error(err),
				zap.Any("ApisixRoute", ar),
			)
			return err
		}

		om := &manifest{
			routes:       oldCtx.Routes,
			upstreams:    oldCtx.Upstreams,
			streamRoutes: oldCtx.StreamRoutes,
		}
		added, updated, deleted = m.diff(om)
	}

	return c.controller.syncManifests(ctx, added, updated, deleted)
}

func (c *apisixRouteController) handleSyncErr(obj interface{}, errOrigin error) {
	ev := obj.(*types.Event)
	event := ev.Object.(kube.ApisixRouteEvent)
	namespace, name, errLocal := cache.SplitMetaNamespaceKey(event.Key)
	if errLocal != nil {
		log.Errorf("invalid resource key: %s", event.Key)
		return
	}
	var ar kube.ApisixRoute
	switch event.GroupVersion {
	case kube.ApisixRouteV1:
		ar, errLocal = c.controller.apisixRouteLister.V1(namespace, name)
	case kube.ApisixRouteV2alpha1:
		ar, errLocal = c.controller.apisixRouteLister.V2alpha1(namespace, name)
	case kube.ApisixRouteV2beta1:
		ar, errLocal = c.controller.apisixRouteLister.V2beta1(namespace, name)
	}
	if errOrigin == nil {
		if ev.Type != types.EventDelete {
			if errLocal == nil {
				switch ar.GroupVersion() {
				case kube.ApisixRouteV1:
					c.controller.recorderEvent(ar.V1(), v1.EventTypeNormal, _resourceSynced, nil)
				case kube.ApisixRouteV2alpha1:
					c.controller.recorderEvent(ar.V2alpha1(), v1.EventTypeNormal, _resourceSynced, nil)
					c.controller.recordStatus(ar.V2alpha1(), _resourceSynced, nil, metav1.ConditionTrue)
				case kube.ApisixRouteV2beta1:
					c.controller.recorderEvent(ar.V2beta1(), v1.EventTypeNormal, _resourceSynced, nil)
					c.controller.recordStatus(ar.V2beta1(), _resourceSynced, nil, metav1.ConditionTrue)
				}
			} else {
				log.Errorw("failed list ApisixRoute",
					zap.Error(errLocal),
					zap.String("name", name),
					zap.String("namespace", namespace),
				)
			}
		}
		c.workqueue.Forget(obj)
		return
	}
	log.Warnw("sync ApisixRoute failed, will retry",
		zap.Any("object", obj),
		zap.Error(errOrigin),
	)
	if errLocal == nil {
		switch ar.GroupVersion() {
		case kube.ApisixRouteV1:
			c.controller.recorderEvent(ar.V1(), v1.EventTypeWarning, _resourceSyncAborted, errOrigin)
		case kube.ApisixRouteV2alpha1:
			c.controller.recorderEvent(ar.V2alpha1(), v1.EventTypeWarning, _resourceSyncAborted, errOrigin)
			c.controller.recordStatus(ar.V2alpha1(), _resourceSyncAborted, errOrigin, metav1.ConditionFalse)
		case kube.ApisixRouteV2beta1:
			c.controller.recorderEvent(ar.V2beta1(), v1.EventTypeWarning, _resourceSyncAborted, errOrigin)
			c.controller.recordStatus(ar.V2beta1(), _resourceSyncAborted, errOrigin, metav1.ConditionFalse)
		}
	} else {
		log.Errorw("failed list ApisixRoute",
			zap.Error(errLocal),
			zap.String("name", name),
			zap.String("namespace", namespace),
		)
	}
	c.workqueue.AddRateLimited(obj)
}

func (c *apisixRouteController) isApisixRouteEffective(ar kube.ApisixRoute) bool {
	var ara string

	if ar.GroupVersion() == kube.ApisixRouteV1 {
		ara = ar.V1().GetAnnotations()[_apisixrouteKey]
	} else if ar.GroupVersion() == kube.ApisixRouteV2alpha1 {
		ara = ar.V2alpha1().GetAnnotations()[_apisixrouteKey]
	} else if ar.GroupVersion() == kube.ApisixRouteV2beta1 {
		ara = ar.V2beta1().GetAnnotations()[_apisixrouteKey]
	} else {
		ara = c.controller.cfg.Kubernetes.ApisixRouteClass
	}

	if c.controller.cfg.Kubernetes.ApisixRouteClass == "" || c.controller.cfg.Kubernetes.ApisixRouteClass == ara {
		return true
	}

	return false
}

func (c *apisixRouteController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found ApisixRoute resource with bad meta namespace key: %s", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}

	ar := kube.MustNewApisixRoute(obj)

	if !c.isApisixRouteEffective(ar) {
		log.Debugw("ignore noneffective ApisixRoute add event",
			zap.Any("object", obj),
		)
		return
	}

	log.Debugw("ApisixRoute add event arrived",
		zap.Any("object", obj))

	c.workqueue.AddRateLimited(&types.Event{
		Type: types.EventAdd,
		Object: kube.ApisixRouteEvent{
			Key:          key,
			GroupVersion: ar.GroupVersion(),
		},
	})
}

func (c *apisixRouteController) onUpdate(oldObj, newObj interface{}) {
	prev := kube.MustNewApisixRoute(oldObj)
	curr := kube.MustNewApisixRoute(newObj)
	if prev.ResourceVersion() >= curr.ResourceVersion() {
		return
	}
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		log.Errorf("found ApisixRoute resource with bad meta namespace key: %s", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	if !c.isApisixRouteEffective(curr) {
		log.Debugw("ignore noneffective ApisixRoute update event arrived",
			zap.Any("new object", curr),
			zap.Any("old object", prev),
		)
		return
	}
	log.Debugw("ApisixRoute update event arrived",
		zap.Any("new object", curr),
		zap.Any("old object", prev),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type: types.EventUpdate,
		Object: kube.ApisixRouteEvent{
			Key:          key,
			GroupVersion: curr.GroupVersion(),
			OldObject:    prev,
		},
	})
}

func (c *apisixRouteController) onDelete(obj interface{}) {
	ar, err := kube.NewApisixRoute(obj)
	if err != nil {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		ar = kube.MustNewApisixRoute(tombstone)
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found ApisixRoute resource with bad meta namesapce key: %s", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	if !c.isApisixRouteEffective(ar) {
		log.Debugw("ignore noneffective ApisixRoute delete event arrived",
			zap.Any("final state", ar),
		)
		return
	}
	log.Debugw("ApisixRoute delete event arrived",
		zap.Any("final state", ar),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type: types.EventDelete,
		Object: kube.ApisixRouteEvent{
			Key:          key,
			GroupVersion: ar.GroupVersion(),
		},
		Tombstone: ar,
	})
}
