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
package kube

import (
	"errors"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	listersnetworkingv1alpha1 "knative.dev/networking/pkg/client/listers/networking/v1alpha1"
)

const (
	// KnativeIngressV1alpha1 represents the KnativeIngress in networking/v1alpha1 group version.
	KnativeIngressV1alpha1 = "networking/v1alpha1"
)

// KnativeIngressLister is an encapsulation for the lister of KnativeIngress,
// it aims at to be compatible with different KnativeIngress versions.
type KnativeIngressLister interface {
	// V1alpha1 gets the ingress in networking/v1alpha1.
	V1alpha1(string, string) (KnativeIngress, error)
}

// KnativeIngressInformer is an encapsulation for the informer of KnativeIngress,
// it aims at to be compatible with different KnativeIngress versions.
type KnativeIngressInformer interface {
	Run(chan struct{})
}

// KnativeIngress is an encapsulation for KnativeIngress with different
// versions, for now, they are networking/v1alpha1.
type KnativeIngress interface {
	// GroupVersion returns the api group version of the
	// real knativeingress.
	GroupVersion() string
	// V1alpha1 returns the knativeingress in networking/v1alpha1, the real
	// knativeingress must be in networking/v1alpha1, or V1alpha1() will panic.
	V1alpha1() *v1alpha1.Ingress
	// ResourceVersion returns the the resource version field inside
	// the real KnativeIngress.
	ResourceVersion() string
}

// KnativeIngressEvents contains the knative ingress key (namespace/name)
// and the group version message.
type KnativeIngressEvent struct {
	Key          string
	GroupVersion string
	OldObject    KnativeIngress
}

type knativeIngress struct {
	groupVersion string
	v1alpha1     *v1alpha1.Ingress
}

func (ing *knativeIngress) V1alpha1() *v1alpha1.Ingress {
	if ing.groupVersion != KnativeIngressV1alpha1 {
		panic("not a networking/v1alpha1 knative ingress")
	}
	return ing.v1alpha1
}

func (ing *knativeIngress) GroupVersion() string {
	return ing.groupVersion
}

func (ing *knativeIngress) ResourceVersion() string {
	if ing.GroupVersion() == KnativeIngressV1alpha1 {
		return ing.V1alpha1().ResourceVersion
	}
	// TODO: In default case, we may return extensions/v1beta1 like in kube/ingress.go
	return ""
}

type knativeIngressLister struct {
	v1alpha1Lister listersnetworkingv1alpha1.IngressLister
}

func (l *knativeIngressLister) V1alpha1(namespace, name string) (KnativeIngress, error) {
	ing, err := l.v1alpha1Lister.Ingresses(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return &knativeIngress{
		groupVersion: KnativeIngressV1alpha1,
		v1alpha1:     ing,
	}, nil
}

// MustNewKnativeIngress creates a kube.KnativeIngress object according to the
// type of obj.
func MustNewKnativeIngress(obj interface{}) KnativeIngress {
	switch ing := obj.(type) {
	case *v1alpha1.Ingress:
		return &knativeIngress{
			groupVersion: KnativeIngressV1alpha1,
			v1alpha1:     ing,
		}
	default:
		panic("invalid knative ingress type")
	}
}

// NewKnativeIngress creates a kube.KnativeIngress object according to the
// type of obj. It returns nil and the error reason when the
// type assertion fails.
func NewKnativeIngress(obj interface{}) (KnativeIngress, error) {
	switch ing := obj.(type) {
	case *v1alpha1.Ingress:
		return &knativeIngress{
			groupVersion: KnativeIngressV1alpha1,
			v1alpha1:     ing,
		}, nil
	default:
		return nil, errors.New("invalid knative ingress type")
	}
}

// NewKnativeIngressLister creates an version-neutral KnativeIngress lister.
func NewKnativeIngressLister(v1alpha1 listersnetworkingv1alpha1.IngressLister) KnativeIngressLister {
	return &knativeIngressLister{
		v1alpha1Lister: v1alpha1,
	}
}
