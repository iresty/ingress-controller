// +build e2e

package conformance

import (
	"testing"

	"knative.dev/networking/test/conformance/ingress"
)

func TestIngressConformance(t *testing.T) {
	ingress.RunConformance(t)
}
