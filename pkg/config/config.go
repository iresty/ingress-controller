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
package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"

	"github.com/apache/apisix-ingress-controller/pkg/types"
)

const (
	// NamespaceAll represents all namespaces.
	NamespaceAll = "*"
	// IngressAPISIXLeader is the default election id for the controller
	// leader election.
	IngressAPISIXLeader = "ingress-apisix-leader"
	// IngressClass is the default ingress class name, used for Ingress
	// object's IngressClassName field in Kubernetes clusters version v1.18.0
	// or higher, or the annotation "kubernetes.io/ingress.class" (deprecated).
	IngressClass = "apisix"

	// IngressNetworkingV1 represents ingress.networking/v1
	IngressNetworkingV1 = "networking/v1"
	// IngressNetworkingV1beta1 represents ingress.networking/v1beta1
	IngressNetworkingV1beta1 = "networking/v1beta1"
	// IngressExtensionsV1beta1 represents ingress.extensions/v1beta1
	// WARNING: ingress.extensions/v1beta1 is deprecated in v1.14+, and will be unavilable
	// in v1.22.
	IngressExtensionsV1beta1 = "extensions/v1beta1"
	// ApisixRouteV1 represents apisixroute.apisix.apache.org/v1
	ApisixRouteV1 = "apisix.apache.org/v1"
	// ApisixRouteV2alpha1 represents apisixroute.apisix.apache.org/v2alpha1
	ApisixRouteV2alpha1 = "apisix.apache.org/v2alpha1"
	// ApisixRouteV2beta1 represents apisixroute.apisix.apache.org/v2beta1
	ApisixRouteV2beta1 = "apisix.apache.org/v2beta1"

	// KnativeIngressNetworkingV1alpha1 represents knative.ingress.networking/v1
	KnativeIngressNetworkingV1alpha1 = "networking/v1alpha1"

	_minimalResyncInterval = 30 * time.Second
)

// Config contains all config items which are necessary for
// apisix-ingress-controller's running.
type Config struct {
	LogLevel        string           `json:"log_level" yaml:"log_level"`
	LogOutput       string           `json:"log_output" yaml:"log_output"`
	HTTPListen      string           `json:"http_listen" yaml:"http_listen"`
	EnableProfiling bool             `json:"enable_profiling" yaml:"enable_profiling"`
	Kubernetes      KubernetesConfig `json:"kubernetes" yaml:"kubernetes"`
	APISIX          APISIXConfig     `json:"apisix" yaml:"apisix"`
}

// KubernetesConfig contains all Kubernetes related config items.
type KubernetesConfig struct {
	Kubeconfig            string             `json:"kubeconfig" yaml:"kubeconfig"`
	ResyncInterval        types.TimeDuration `json:"resync_interval" yaml:"resync_interval"`
	AppNamespaces         []string           `json:"app_namespaces" yaml:"app_namespaces"`
	ElectionID            string             `json:"election_id" yaml:"election_id"`
	IngressClass          string             `json:"ingress_class" yaml:"ingress_class"`
	IngressVersion        string             `json:"ingress_version" yaml:"ingress_version"`
	WatchEndpointSlices   bool               `json:"watch_endpoint_slices" yaml:"watch_endpoint_slices"`
	ApisixRouteVersion    string             `json:"apisix_route_version" yaml:"apisix_route_version"`
	KnativeIngressVersion string             `json:"knative_ingress_version" yaml:"knative_ingress_version"`
}

// APISIXConfig contains all APISIX related config items.
type APISIXConfig struct {
	// DefaultClusterName is the name of default cluster.
	DefaultClusterName string `json:"default_cluster_name"`
	// DefaultClusterBaseURL is the base url configuration for the default cluster.
	DefaultClusterBaseURL string `json:"default_cluster_base_url" yaml:"default_cluster_base_url"`
	// DefaultClusterAdminKey is the admin key for the default cluster.
	// TODO: Obsolete the plain way to specify admin_key, which is insecure.
	DefaultClusterAdminKey string `json:"default_cluster_admin_key" yaml:"default_cluster_admin_key"`
	// BaseURL is same to DefaultClusterBaseURL.
	// Deprecated: use DefaultClusterBaseURL instead. BaseURL will be removed
	// once v1.0.0 is released.
	BaseURL string `json:"base_url" yaml:"base_url"`
	// AdminKey is same to DefaultClusterAdminKey.
	// Deprecated: use DefaultClusterAdminKey instead. AdminKey will be removed
	// once v1.0.0 is released.
	AdminKey string `json:"admin_key" yaml:"admin_key"`
}

// NewDefaultConfig creates a Config object which fills all config items with
// default value.
func NewDefaultConfig() *Config {
	return &Config{
		LogLevel:        "warn",
		LogOutput:       "stderr",
		HTTPListen:      ":8080",
		EnableProfiling: true,
		Kubernetes: KubernetesConfig{
			Kubeconfig:          "", // Use in-cluster configurations.
			ResyncInterval:      types.TimeDuration{Duration: 6 * time.Hour},
			AppNamespaces:       []string{v1.NamespaceAll},
			ElectionID:          IngressAPISIXLeader,
			IngressClass:        IngressClass,
			IngressVersion:      IngressNetworkingV1,
			ApisixRouteVersion:  ApisixRouteV2alpha1,
			WatchEndpointSlices: false,
			KnativeIngressVersion: KnativeIngressNetworkingV1alpha1,
		},
	}
}

// NewConfigFromFile creates a Config object and fills all config items according
// to the configuration file. The file can be in JSON/YAML format, which will be
// distinguished according to the file suffix.
func NewConfigFromFile(filename string) (*Config, error) {
	cfg := NewDefaultConfig()
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		err = yaml.Unmarshal(data, cfg)
	} else {
		err = json.Unmarshal(data, cfg)
	}

	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate validates whether the Config is right.
func (cfg *Config) Validate() error {
	if cfg.Kubernetes.ResyncInterval.Duration < _minimalResyncInterval {
		return errors.New("controller resync interval too small")
	}
	if cfg.APISIX.DefaultClusterAdminKey == "" {
		cfg.APISIX.DefaultClusterAdminKey = cfg.APISIX.AdminKey
	}
	if cfg.APISIX.DefaultClusterBaseURL == "" {
		cfg.APISIX.DefaultClusterBaseURL = cfg.APISIX.BaseURL
	}
	if cfg.APISIX.DefaultClusterName == "" {
		cfg.APISIX.DefaultClusterName = "default"
	}

	if cfg.APISIX.DefaultClusterBaseURL == "" {
		return errors.New("apisix base url is required")
	}
	switch cfg.Kubernetes.IngressVersion {
	case IngressNetworkingV1, IngressNetworkingV1beta1, IngressExtensionsV1beta1:
		break
	default:
		return errors.New("unsupported ingress version")
	}
	switch cfg.Kubernetes.KnativeIngressVersion {
	case KnativeIngressNetworkingV1alpha1:
		break
	default:
		return errors.New("unsupported knative ingress version")
	}
	cfg.Kubernetes.AppNamespaces = purifyAppNamespaces(cfg.Kubernetes.AppNamespaces)
	return nil
}

func purifyAppNamespaces(namespaces []string) []string {
	exists := make(map[string]struct{})
	var ultimate []string
	for _, ns := range namespaces {
		if ns == NamespaceAll {
			return []string{v1.NamespaceAll}
		}
		if _, ok := exists[ns]; !ok {
			ultimate = append(ultimate, ns)
			exists[ns] = struct{}{}
		}
	}
	return ultimate
}
