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
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/apache/apisix-ingress-controller/pkg/kube"
	configv1 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v1"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	"github.com/apache/apisix-ingress-controller/pkg/types"
	v1 "github.com/apache/apisix-ingress-controller/pkg/types/apisix/v1"
)

const _tlsController = "TlsController"

type apisixTlsController struct {
	controller *Controller
	workqueue  workqueue.RateLimitingInterface
	workers    int
	recorder   record.EventRecorder
}

func (c *Controller) newApisixTlsController() *apisixTlsController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kube.GetKubeClient().CoreV1().Events("")})
	ctl := &apisixTlsController{
		controller: c,
		workqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(1*time.Second, 60*time.Second, 5), "ApisixTls"),
		workers:    1,
		recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: _tlsController}),
	}
	ctl.controller.apisixTlsInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ctl.onAdd,
			UpdateFunc: ctl.onUpdate,
			DeleteFunc: ctl.onDelete,
		},
	)
	return ctl
}

func (c *apisixTlsController) run(ctx context.Context) {
	log.Info("ApisixTls controller started")
	defer log.Info("ApisixTls controller exited")
	if ok := cache.WaitForCacheSync(ctx.Done(), c.controller.apisixTlsInformer.HasSynced, c.controller.secretInformer.HasSynced); !ok {
		log.Errorf("informers sync failed")
		return
	}
	for i := 0; i < c.workers; i++ {
		go c.runWorker(ctx)
	}

	<-ctx.Done()
	c.workqueue.ShutDown()
}

func (c *apisixTlsController) runWorker(ctx context.Context) {
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

func (c *apisixTlsController) sync(ctx context.Context, ev *types.Event) error {
	key := ev.Object.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.Errorf("found ApisixTls resource with invalid meta namespace key %s: %s", key, err)
		return err
	}

	tls, err := c.controller.apisixTlsLister.ApisixTlses(namespace).Get(name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			log.Errorf("failed to get ApisixTls %s: %s", key, err)
			return err
		}
		if ev.Type != types.EventDelete {
			log.Warnf("ApisixTls %s was deleted before it can be delivered", key)
			// Don't need to retry.
			return nil
		}
	}
	if ev.Type == types.EventDelete {
		if tls != nil {
			// We still find the resource while we are processing the DELETE event,
			// that means object with same namespace and name was created, discarding
			// this stale DELETE event.
			log.Warnf("discard the stale ApisixTls delete event since the %s exists", key)
			return nil
		}
		tls = ev.Tombstone.(*configv1.ApisixTls)
	}

	ssl, err := c.controller.translator.TranslateSSL(tls)
	if err != nil {
		log.Errorw("failed to translate ApisixTls",
			zap.Error(err),
			zap.Any("ApisixTls", tls),
		)
		message := fmt.Sprintf(_messageResourceFailed, _tlsController, err.Error())
		c.recorder.Event(tls, corev1.EventTypeWarning, _resourceSyncAborted, message)
		return err
	}
	log.Debug("got SSL object from ApisixTls",
		zap.Any("ssl", ssl),
		zap.Any("ApisixTls", tls),
	)

	secretKey := tls.Spec.Secret.Namespace + "_" + tls.Spec.Secret.Name
	c.syncSecretSSL(secretKey, ssl, ev.Type)

	if err := c.controller.syncSSL(ctx, ssl, ev.Type); err != nil {
		log.Errorw("failed to sync SSL to APISIX",
			zap.Error(err),
			zap.Any("ssl", ssl),
		)
		message := fmt.Sprintf(_messageResourceFailed, _tlsController, err.Error())
		c.recorder.Event(tls, corev1.EventTypeWarning, _resourceSyncAborted, message)
		return err
	}
	message := fmt.Sprintf(_messageResourceSynced, _tlsController)
	c.recorder.Event(tls, corev1.EventTypeNormal, _resourceSynced, message)
	return err
}

func (c *apisixTlsController) syncSecretSSL(key string, ssl *v1.Ssl, event types.EventType) {
	if ssls, ok := c.controller.secretSSLMap.Load(key); ok {
		sslMap := ssls.(*sync.Map)
		switch event {
		case types.EventDelete:
			sslMap.Delete(ssl.ID)
			c.controller.secretSSLMap.Store(key, sslMap)
		default:
			sslMap.Store(ssl.ID, ssl)
			c.controller.secretSSLMap.Store(key, sslMap)
		}
	} else if event != types.EventDelete {
		sslMap := new(sync.Map)
		sslMap.Store(ssl.ID, ssl)
		c.controller.secretSSLMap.Store(key, sslMap)
	}
}

func (c *apisixTlsController) handleSyncErr(obj interface{}, err error) {
	if err == nil {
		c.workqueue.Forget(obj)
		return
	}
	log.Warnw("sync ApisixTls failed, will retry",
		zap.Any("object", obj),
		zap.Error(err),
	)
	c.workqueue.AddRateLimited(obj)
}

func (c *apisixTlsController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found ApisixTls object with bad namespace/name: %s, ignore it", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	log.Debugw("ApisixTls add event arrived",
		zap.Any("object", obj),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type:   types.EventAdd,
		Object: key,
	})
}

func (c *apisixTlsController) onUpdate(prev, curr interface{}) {
	oldTls := prev.(*configv1.ApisixTls)
	newTls := curr.(*configv1.ApisixTls)
	if oldTls.GetResourceVersion() == newTls.GetResourceVersion() {
		return
	}
	key, err := cache.MetaNamespaceKeyFunc(curr)
	if err != nil {
		log.Errorf("found ApisixTls object with bad namespace/name: %s, ignore it", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	log.Debugw("ApisixTls update event arrived",
		zap.Any("new object", curr),
		zap.Any("old object", prev),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type:   types.EventUpdate,
		Object: key,
	})
}

func (c *apisixTlsController) onDelete(obj interface{}) {
	tls, ok := obj.(*configv1.ApisixTls)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		tls, ok = tombstone.Obj.(*configv1.ApisixTls)
		if !ok {
			return
		}
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found ApisixTls resource with bad meta namespace key: %s", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	log.Debugw("ApisixTls delete event arrived",
		zap.Any("final state", obj),
	)
	c.workqueue.AddRateLimited(&types.Event{
		Type:      types.EventDelete,
		Object:    key,
		Tombstone: tls,
	})
}
