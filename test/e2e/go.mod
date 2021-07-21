module github.com/apache/apisix-ingress-controller/test/e2e

go 1.14

require (
	github.com/apache/apisix-ingress-controller v0.0.0-20210105024109-72e53386de5a
	github.com/gavv/httpexpect/v2 v2.2.0
	github.com/gorilla/websocket v1.4.2
	github.com/gruntwork-io/terratest v0.32.8
	github.com/onsi/ginkgo v1.16.4
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
)

replace github.com/apache/apisix-ingress-controller => ../../
