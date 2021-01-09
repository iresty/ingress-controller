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
package controller

import (
	"fmt"
	"time"

	apisixV1 "github.com/gxthrj/apisix-ingress-types/pkg/apis/config/v1"
	clientSet "github.com/gxthrj/apisix-ingress-types/pkg/client/clientset/versioned"
	apisixScheme "github.com/gxthrj/apisix-ingress-types/pkg/client/clientset/versioned/scheme"
	informers "github.com/gxthrj/apisix-ingress-types/pkg/client/informers/externalversions/config/v1"
	"github.com/gxthrj/apisix-ingress-types/pkg/client/listers/config/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/api7/ingress-controller/pkg/ingress/apisix"
	"github.com/api7/ingress-controller/pkg/ingress/endpoint"
	"github.com/api7/ingress-controller/pkg/log"
	"github.com/api7/ingress-controller/pkg/seven/state"
)

type ApisixUpstreamController struct {
	kubeclientset        kubernetes.Interface
	apisixClientset      clientSet.Interface
	apisixUpstreamList   v1.ApisixUpstreamLister
	apisixUpstreamSynced cache.InformerSynced
	workqueue            workqueue.RateLimitingInterface
}

func BuildApisixUpstreamController(
	kubeclientset kubernetes.Interface,
	apisixUpstreamClientset clientSet.Interface,
	apisixUpstreamInformer informers.ApisixUpstreamInformer) *ApisixUpstreamController {

	runtime.Must(apisixScheme.AddToScheme(scheme.Scheme))
	controller := &ApisixUpstreamController{
		kubeclientset:        kubeclientset,
		apisixClientset:      apisixUpstreamClientset,
		apisixUpstreamList:   apisixUpstreamInformer.Lister(),
		apisixUpstreamSynced: apisixUpstreamInformer.Informer().HasSynced,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(1*time.Second, 60*time.Second, 5), "ApisixUpstreams"),
	}
	apisixUpstreamInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.addFunc,
			UpdateFunc: controller.updateFunc,
			DeleteFunc: controller.deleteFunc,
		})
	return controller
}

func (c *ApisixUpstreamController) Run(stop <-chan struct{}) error {
	// 同步缓存
	if ok := cache.WaitForCacheSync(stop); !ok {
		log.Error("同步ApisixUpstream缓存失败")
		return fmt.Errorf("failed to wait for caches to sync")
	}
	go wait.Until(c.runWorker, time.Second, stop)
	return nil
}

func (c *ApisixUpstreamController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ApisixUpstreamController) processNextWorkItem() bool {
	defer recoverException()
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var sqo *UpstreamQueueObj
		var ok bool

		if sqo, ok = obj.(*UpstreamQueueObj); !ok {
			c.workqueue.Forget(obj)
			return fmt.Errorf("expected string in workqueue but got %#v", obj)
		}
		// 在syncHandler中处理业务
		if err := c.syncHandler(sqo); err != nil {
			c.workqueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing '%s': %s", sqo.Key, err.Error())
		}

		c.workqueue.Forget(obj)
		return nil
	}(obj)
	if err != nil {
		runtime.HandleError(err)
	}
	return true
}

func (c *ApisixUpstreamController) syncHandler(sqo *UpstreamQueueObj) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(sqo.Key)
	if err != nil {
		log.Errorf("invalid resource key: %s", sqo.Key)
		return fmt.Errorf("invalid resource key: %s", sqo.Key)
	}
	apisixUpstreamYaml := sqo.OldObj
	if sqo.Ope == DELETE {
		apisixIngressUpstream, _ := c.apisixUpstreamList.ApisixUpstreams(namespace).Get(name)
		if apisixIngressUpstream != nil && apisixIngressUpstream.ResourceVersion > sqo.OldObj.ResourceVersion {
			log.Warnf("Upstream %s has been covered when retry", sqo.Key)
			return nil
		}
	} else {
		apisixUpstreamYaml, err = c.apisixUpstreamList.ApisixUpstreams(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Infof("apisixUpstream %s is removed", sqo.Key)
				return nil
			}
			runtime.HandleError(fmt.Errorf("failed to list apisixUpstream %s/%s", sqo.Key, err.Error()))
			return err
		}
	}
	aub := apisix.ApisixUpstreamBuilder{CRD: apisixUpstreamYaml, Ep: &endpoint.EndpointRequest{}}
	upstreams, _ := aub.Convert()
	comb := state.ApisixCombination{Routes: nil, Services: nil, Upstreams: upstreams}
	if sqo.Ope == DELETE {
		return comb.Remove()
	} else {
		_, err = comb.Solver()
		return err
	}

}

type UpstreamQueueObj struct {
	Key    string                   `json:"key"`
	OldObj *apisixV1.ApisixUpstream `json:"old_obj"`
	Ope    string                   `json:"ope"` // add / update / delete
}

func (c *ApisixUpstreamController) addFunc(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	sqo := &UpstreamQueueObj{Key: key, OldObj: nil, Ope: ADD}
	c.workqueue.AddRateLimited(sqo)
}

func (c *ApisixUpstreamController) updateFunc(oldObj, newObj interface{}) {
	oldUpstream := oldObj.(*apisixV1.ApisixUpstream)
	newUpstream := newObj.(*apisixV1.ApisixUpstream)
	if oldUpstream.ResourceVersion >= newUpstream.ResourceVersion {
		return
	}
	var (
		key string
		err error
	)
	if key, err = cache.MetaNamespaceKeyFunc(newObj); err != nil {
		runtime.HandleError(err)
		return
	}
	sqo := &UpstreamQueueObj{Key: key, OldObj: oldUpstream, Ope: UPDATE}
	c.addFunc(sqo)
}

func (c *ApisixUpstreamController) deleteFunc(obj interface{}) {
	oldUpstream, ok := obj.(*apisixV1.ApisixUpstream)
	if !ok {
		oldState, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		oldUpstream, ok = oldState.Obj.(*apisixV1.ApisixUpstream)
		if !ok {
			return
		}
	}
	var key string
	var err error
	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}
	sqo := &UpstreamQueueObj{Key: key, OldObj: oldUpstream, Ope: DELETE}
	c.workqueue.AddRateLimited(sqo)
}
