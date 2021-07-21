// +build e2e

package e2eknative

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/networking/test"
	"knative.dev/networking/test/conformance/ingress"
)

func TestConflictingDomains(t *testing.T) {
	clients := test.Setup(t)
	ctx := context.Background()

	name, port, _ := ingress.CreateRuntimeService(ctx, t, clients, networking.ServicePortNameHTTP1)

	spec := v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Hosts:      []string{name + ".example.com"},
			Visibility: v1alpha1.IngressVisibilityExternalIP,
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      name,
							ServiceNamespace: test.ServingNamespace,
							ServicePort:      intstr.FromInt(port),
						},
					}},
				}},
			},
		}},
	}

	// The first ingress should become ready just fine.
	ingress.CreateIngressReady(ctx, t, clients, spec)

	// The second one with the same spec is supposed to throw a conflict error.
	ing, _ := ingress.CreateIngress(ctx, t, clients, spec)
	if err := ingress.WaitForIngressState(
		ctx,
		clients.NetworkingClient,
		ing.Name,
		func(r *v1alpha1.Ingress) (bool, error) {
			return r.GetStatus().GetCondition(v1alpha1.IngressConditionLoadBalancerReady).GetReason() == "DomainConflict", nil
		},
		t.Name()); err != nil {
		t.Fatalf("Error waiting for ingress %q state: %v", ing.Name, err)
	}

}
