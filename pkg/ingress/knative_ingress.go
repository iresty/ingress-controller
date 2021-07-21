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
	"net"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	apiv1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	knative "knative.dev/networking/pkg/apis/networking/v1alpha1"
	knativeApis "knative.dev/pkg/apis"
	"knative.dev/pkg/network"

	"github.com/apache/apisix-ingress-controller/pkg/kube"
	"github.com/apache/apisix-ingress-controller/pkg/log"
	"github.com/apache/apisix-ingress-controller/pkg/types"
)

//const knativeIngressClassKey = "networking.knative.dev/ingress.class"

type knativeIngressController struct {
	controller *Controller
	workqueue  workqueue.RateLimitingInterface
	workers    int
}

var ingressCondSet = knativeApis.NewLivingConditionSet()

func (c *Controller) newKnativeIngressController() *knativeIngressController {
	ctl := &knativeIngressController{
		controller: c,
		workqueue:  workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(1*time.Second, 60*time.Second, 5), "KnativeIngress"),
		workers:    1,
	}

	c.knativeIngressInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ctl.onAdd,
		UpdateFunc: ctl.onUpdate,
		DeleteFunc: ctl.OnDelete,
	})
	return ctl
}

func (c *knativeIngressController) run(ctx context.Context) {
	log.Info("knative ingress controller started")
	defer log.Infof("knative ingress controller exited")
	defer c.workqueue.ShutDown()

	if !cache.WaitForCacheSync(ctx.Done(), c.controller.knativeIngressInformer.HasSynced) {
		log.Errorf("cache sync failed")
		return
	}
	for i := 0; i < c.workers; i++ {
		go c.runWorker(ctx)
	}
	<-ctx.Done()
}

func (c *knativeIngressController) runWorker(ctx context.Context) {
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

func (c *knativeIngressController) sync(ctx context.Context, ev *types.Event) error {
	ingEv := ev.Object.(kube.KnativeIngressEvent)
	namespace, name, err := cache.SplitMetaNamespaceKey(ingEv.Key)
	if err != nil {
		log.Errorf("found knative ingress resource with invalid meta namespace key %s: %s", ingEv.Key, err)
		return err
	}

	var ing kube.KnativeIngress
	switch ingEv.GroupVersion {
	case kube.KnativeIngressV1alpha1:
		ing, err = c.controller.knativeIngressLister.V1alpha1(namespace, name)
	default:
		err = fmt.Errorf("unsupported group version %s, one of (%s) is expected", ingEv.GroupVersion,
			kube.KnativeIngressV1alpha1)
	}

	if err != nil {
		if !k8serrors.IsNotFound(err) { // TODO: Not sure whether we should use this k8serrors or not
			log.Errorf("failed to get knative ingress %s (group version: %s): %s", ingEv.Key, ingEv.GroupVersion, err)
			return err
		}

		if ev.Type != types.EventDelete {
			log.Warnf("knative ingress %s (group version: %s) was deleted before it can be delivered", ingEv.Key, ingEv.GroupVersion)
			// Don't need to retry.
			return nil
		}
	}
	if ev.Type == types.EventDelete {
		if ing != nil {
			// We still find the resource while we are processing the DELETE event,
			// that means object with same namespace and name was created, discarding
			// this stale DELETE event.
			log.Warnf("discard the stale knative ingress delete event since the %s exists", ingEv.Key)
			return nil
		}
		ing = ev.Tombstone.(kube.KnativeIngress)
	}

	tctx, err := c.controller.translator.TranslateKnativeIngress(ing)
	if err != nil {
		log.Errorw("failed to translate knative ingress",
			zap.Error(err),
			zap.Any("knative_ingress", ing),
		)
		return err
	}

	log.Debugw("translated knative ingress resource to a couple of routes and upstreams",
		zap.Any("knative_ingress", ing),
		zap.Any("routes", tctx.Routes),
		zap.Any("upstreams", tctx.Upstreams),
	)

	m := &manifest{
		routes:    tctx.Routes,
		upstreams: tctx.Upstreams,
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
		oldCtx, err := c.controller.translator.TranslateKnativeIngress(ingEv.OldObject)
		if err != nil {
			log.Errorw("failed to translate knative ingress",
				zap.String("event", "update"),
				zap.Error(err),
				zap.Any("knative_ingress", ingEv.OldObject),
			)
			return err
		}
		om := &manifest{
			routes:    oldCtx.Routes,
			upstreams: oldCtx.Upstreams,
		}
		added, updated, deleted = m.diff(om)
	}
	if err := c.controller.syncManifests(ctx, added, updated, deleted); err != nil {
		log.Errorw("failed to sync knative ingress artifacts",
			zap.Error(err),
		)
		return err
	}

	addrs, err := c.runningAddresses(ctx)
	if err != nil {
		return err
	}
	log.Infow("output of runningAddresses()",
		zap.Any("addrs", addrs))
	status := sliceToStatus(addrs)
	log.Infow("output of sliceToStatus()",
		zap.Any("status", status))
	switch ing.GroupVersion() {
	case kube.KnativeIngressV1alpha1:
		currIng := ing.V1alpha1()
		log.Infow("begin syncing Knative Ingress",
			zap.String("ingress_namespace", currIng.Namespace),
			zap.String("ingress_name", currIng.Name))
		sort.SliceStable(status, lessLoadBalancerIngress(status)) // BUG: data race - see issue #829
		curIPs := toCoreLBStatus(currIng.Status.PublicLoadBalancer)
		sort.SliceStable(curIPs, lessLoadBalancerIngress(curIPs))

		ingClient := c.controller.kubeClient.KnativeClient.NetworkingV1alpha1().Ingresses(namespace)

		if ingressSliceEqual(status, curIPs) &&
			currIng.Status.ObservedGeneration == currIng.GetObjectMeta().GetGeneration() {
			log.Infow("no change in status, update skipped")
			return nil
		}

		log.Infow("attempting to update Knative Ingress status", zap.Any("ingress_status", status))
		lbStatus := toKnativeLBStatus(status)

		svcNamespace, svcName := "ingress-apisix", "apisix-gateway"
		clusterDomain := network.GetClusterDomainName()

		for i := 0; i < len(lbStatus); i++ {
			lbStatus[i].DomainInternal = fmt.Sprintf("%s.%s.svc.%s",
				svcName, svcNamespace, clusterDomain)
		}

		currIng.Status.MarkLoadBalancerReady(lbStatus, lbStatus)
		ingressCondSet.Manage(&currIng.Status).MarkTrue(knative.IngressConditionReady)
		ingressCondSet.Manage(&currIng.Status).MarkTrue(knative.IngressConditionNetworkConfigured)
		currIng.Status.ObservedGeneration = currIng.GetObjectMeta().GetGeneration()

		_, err = ingClient.UpdateStatus(ctx, currIng, metav1.UpdateOptions{})
		if err != nil {
			log.Errorf("failed to update ingress status: %v", err)
		} else {
			log.Debugw("successfully updated ingress status",
				zap.Any("Knative Ingress", currIng))
		}
	default:
		err = fmt.Errorf("unsupported group version %s, one of (%s) is expected", ing.GroupVersion(),
			kube.KnativeIngressV1alpha1)
	}
	return nil
}

func (c *knativeIngressController) handleSyncErr(obj interface{}, err error) {
	if err == nil {
		c.workqueue.Forget(obj)
		return
	}
	log.Warnw("sync knative ingress failed, will retry",
		zap.Any("object", obj),
		zap.Error(err),
	)
	c.workqueue.AddRateLimited(obj)
}

func (c *knativeIngressController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found knative ingress resource with bad meta namespace key: %s", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}

	ing := kube.MustNewKnativeIngress(obj)
	valid := c.isKnativeIngressEffective(ing)
	if valid {
		log.Debugw("knative ingress add event arrived",
			zap.Any("object", ing),
		)
	} else {
		log.Debugw("ignore noneffective knative ingress add event",
			zap.Any("object", ing),
		)
		return
	}

	c.workqueue.AddRateLimited(&types.Event{
		Type: types.EventAdd,
		Object: kube.KnativeIngressEvent{
			Key:          key,
			GroupVersion: ing.GroupVersion(),
		},
	})
}

func (c *knativeIngressController) onUpdate(oldObj, newObj interface{}) {
	prev := kube.MustNewKnativeIngress(oldObj)
	curr := kube.MustNewKnativeIngress(newObj)
	if prev.ResourceVersion() >= curr.ResourceVersion() {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		log.Errorf("found knative ingress resource with bad meta namespace key: %s", err)
		return
	}
	valid := c.isKnativeIngressEffective(curr)
	if valid {
		log.Debugw("knative ingress update event arrived",
			zap.Any("new object", oldObj),
			zap.Any("old object", newObj),
		)
	} else {
		log.Debugw("ignore noneffective knative  ingress update event",
			zap.Any("new object", oldObj),
			zap.Any("old object", newObj),
		)
		return
	}

	c.workqueue.AddRateLimited(&types.Event{
		Type: types.EventUpdate,
		Object: kube.KnativeIngressEvent{
			Key:          key,
			GroupVersion: curr.GroupVersion(),
			OldObject:    prev,
		},
	})
}

func (c *knativeIngressController) OnDelete(obj interface{}) {
	ing, err := kube.NewKnativeIngress(obj)
	if err != nil {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		ing = kube.MustNewKnativeIngress(tombstone)
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Errorf("found knative ingress resource with bad meta namespace key: %s", err)
		return
	}
	if !c.controller.namespaceWatching(key) {
		return
	}
	valid := c.isKnativeIngressEffective(ing)
	if valid {
		log.Debugw("knative ingress delete event arrived",
			zap.Any("final state", ing),
		)
	} else {
		log.Debugw("ignore noneffective knative ingress delete event",
			zap.Any("object", ing),
		)
		return
	}
	c.workqueue.AddRateLimited(&types.Event{
		Type: types.EventDelete,
		Object: kube.KnativeIngressEvent{
			Key:          key,
			GroupVersion: ing.GroupVersion(),
		},
		Tombstone: ing,
	})
}

func (c *knativeIngressController) isKnativeIngressEffective(ing kube.KnativeIngress) bool {
	// TODO: add Knative ingress effective check
	return true
	//var (
	//	ic  *string
	//	ica string
	//)
	//if ing.GroupVersion() == kube.KnativeIngressV1alpha1 {
	//	ic = ing.V1alpha1().Spec.IngressClassName
	//	ica = ing.V1alpha1().GetAnnotations()[_ingressKey]
	//}
	//
	//
	//// kubernetes.io/ingress.class takes the precedence.
	//// WARN: Cur default ica and ic are all from k8s, which should be replaced by knative
	//if ica != "" {
	//	return ica == c.controller.cfg.Kubernetes.IngressClass
	//}
	//if ic != nil {
	//	return *ic == c.controller.cfg.Kubernetes.IngressClass
	//}
	//
	//return false
}

// From Kong store.go
//func (c *knativeIngressController) isKnativeIngressEffective(objectMeta *metav1.ObjectMeta) bool {
//	ingressAnnotationValue := objectMeta.GetAnnotations()[knativeIngressClassKey]
//	return ingressAnnotationValue == s.ingressClass
//}

// runningAddresses returns a list of IP addresses and/or FQDN where the
// ingress controller is currently running
func (c *knativeIngressController) runningAddresses(ctx context.Context) ([]string, error) {
	addrs := []string{}
	coreClient := c.controller.kubeClient.Client

	// TODO: Not sure whether he name should be ingress-controller service while it does not exist, so replace it with gateway
	// It should fall into the default case
	ns, name := "ingress-apisix", "apisix-gateway"
	svc, err := coreClient.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	switch svc.Spec.Type {
	case apiv1.ServiceTypeLoadBalancer:
		for _, ip := range svc.Status.LoadBalancer.Ingress {
			if ip.IP == "" {
				addrs = append(addrs, ip.Hostname)
			} else {
				addrs = append(addrs, ip.IP)
			}
		}

		addrs = append(addrs, svc.Spec.ExternalIPs...)
		return addrs, nil
	default:
		//podNs := c.controller.namespace
		podNs := "ingress-apisix"
		// TODO: should we get information about all the pods running the "ingress controller"?
		// or instead we should get pods running "apisix-gateway"
		podsList, err := coreClient.CoreV1().Pods(podNs).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		log.Infow("In runningAddresses(): ",
			zap.Any("podsList", podsList))
		var pods []*apiv1.Pod
		log.Infow("In runningAddresses(), begin iterating pod: ")
		for _, currPod := range podsList.Items {
			if strings.Contains(currPod.ObjectMeta.Name, "apisix-etcd") {
				continue
			}
			if strings.Contains(currPod.ObjectMeta.Name, "apisix-") {
				log.Infow("In runningAddresses(), matched pod: ",
					zap.String("currPod_name", currPod.ObjectMeta.Name))
				pods = append(pods, &currPod)
			}
		}
		for _, pod := range pods {
			// only Running pods are valid
			if pod.Status.Phase != apiv1.PodRunning {
				continue
			}

			name := GetNodeIPOrName(ctx, coreClient, pod.Spec.NodeName)
			if !inSlice(name, addrs) {
				addrs = append(addrs, name)
			}
		}
		/*
			podName := c.controller.name
			pod, err := coreClient.CoreV1().Pods(podNs).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			labelSet := pod.GetLabels()
			pods, err := coreClient.CoreV1().Pods(podNs).List(ctx, metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelSet).String(),
			})
			if err != nil {
				return nil, err
			}

			for _, pod := range pods.Items {
				// only Running pods are valid
				if pod.Status.Phase != apiv1.PodRunning {
					continue
				}

				name := GetNodeIPOrName(ctx, coreClient, pod.Spec.NodeName)
				if !inSlice(name, addrs) {
					addrs = append(addrs, name)
				}
			}
		*/

		return addrs, nil
	}
}

func inSlice(e string, arr []string) bool {
	for _, v := range arr {
		if v == e {
			return true
		}
	}

	return false
}

// sliceToStatus converts a slice of IP and/or hostnames to LoadBalancerIngress
func sliceToStatus(endpoints []string) []apiv1.LoadBalancerIngress {
	lbi := []apiv1.LoadBalancerIngress{}
	for _, ep := range endpoints {
		if net.ParseIP(ep) == nil {
			lbi = append(lbi, apiv1.LoadBalancerIngress{Hostname: ep})
		} else {
			lbi = append(lbi, apiv1.LoadBalancerIngress{IP: ep})
		}
	}

	sort.SliceStable(lbi, func(a, b int) bool {
		return lbi[a].IP < lbi[b].IP
	})

	return lbi
}

func toCoreLBStatus(knativeLBStatus *knative.LoadBalancerStatus) []apiv1.LoadBalancerIngress {
	var res []apiv1.LoadBalancerIngress
	if knativeLBStatus == nil {
		return res
	}
	for _, status := range knativeLBStatus.Ingress {
		res = append(res, apiv1.LoadBalancerIngress{
			IP:       status.IP,
			Hostname: status.Domain,
		})
	}
	return res
}

func toKnativeLBStatus(coreLBStatus []apiv1.LoadBalancerIngress) []knative.LoadBalancerIngressStatus {
	var res []knative.LoadBalancerIngressStatus
	for _, status := range coreLBStatus {
		res = append(res, knative.LoadBalancerIngressStatus{
			IP:     status.IP,
			Domain: status.Hostname,
		})
	}
	return res
}

func lessLoadBalancerIngress(addrs []apiv1.LoadBalancerIngress) func(int, int) bool {
	return func(a, b int) bool {
		switch strings.Compare(addrs[a].Hostname, addrs[b].Hostname) {
		case -1:
			return true
		case 1:
			return false
		}
		return addrs[a].IP < addrs[b].IP
	}
}

func ingressSliceEqual(lhs, rhs []apiv1.LoadBalancerIngress) bool {
	if len(lhs) != len(rhs) {
		return false
	}

	for i := range lhs {
		if lhs[i].IP != rhs[i].IP {
			return false
		}
		if lhs[i].Hostname != rhs[i].Hostname {
			return false
		}
	}
	return true
}

// GetNodeIPOrName returns the IP address or the name of a node in the cluster
func GetNodeIPOrName(ctx context.Context, kubeClient clientset.Interface, name string) string {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ""
	}

	ip := ""

	for _, address := range node.Status.Addresses {
		if address.Type == apiv1.NodeExternalIP {
			if address.Address != "" {
				ip = address.Address
				break
			}
		}
	}

	// Report the external IP address of the node
	if ip != "" {
		return ip
	}

	for _, address := range node.Status.Addresses {
		if address.Type == apiv1.NodeInternalIP {
			if address.Address != "" {
				ip = address.Address
				break
			}
		}
	}

	return ip
}
