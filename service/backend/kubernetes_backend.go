// Copyright (c) 2016 Pulcy.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	k8s "github.com/YakLabs/k8s-client"
	"github.com/YakLabs/k8s-client/http"
	"github.com/op/go-logging"
	regapi "github.com/pulcy/registrator-api"
	api "github.com/pulcy/robin-api"
)

const (
	RobinFrontendRecordAnnotationKey = "pulcy.com.robin.frontend.record"
)

type k8sBackend struct {
	config         BackendConfig
	client         k8s.Client
	Logger         *logging.Logger
	lastKnownState string
}

func NewKubernetesBackend(config BackendConfig, logger *logging.Logger) (Backend, error) {
	client, err := http.NewInCluster()
	if err != nil {
		return nil, maskAny(err)
	}
	return &k8sBackend{
		config: config,
		client: client,
		Logger: logger,
	}, nil
}

// Watch for changes on a path and return where there is a change.
func (eb *k8sBackend) Watch() error {
	for {
		list, err := eb.Services()
		if err != nil {
			return maskAny(err)
		}
		state := list.FullString()
		if state == eb.lastKnownState {
			time.Sleep(time.Second * 30)
		} else {
			return nil
		}
	}
}

// Load all registered services
func (eb *k8sBackend) Services() (ServiceRegistrations, error) {
	ingresses, err := eb.listIngresses()
	if err != nil {
		return nil, maskAny(err)
	}
	result := ServiceRegistrations{}
	for _, i := range ingresses {
		srs, err := eb.createServiceRegistrationsFromIngress(i)
		if err != nil {
			return nil, maskAny(err)
		}
		result = append(result, srs...)
	}

	return result, nil
}

// createServiceRegistrationsFromIngress creates all ServiceRegistrations needed for the given ingress.
func (eb *k8sBackend) createServiceRegistrationsFromIngress(i k8s.Ingress) (ServiceRegistrations, error) {
	// Look for FrontendRecord annotation
	raw, found := i.GetAnnotations()[RobinFrontendRecordAnnotationKey]
	if found {
		var frontendRecord api.FrontendRecord
		if err := json.Unmarshal([]byte(raw), &frontendRecord); err != nil {
			return nil, maskAny(err)
		}
		result, err := eb.createServiceRegistrationsFromFrontendRecord(i, frontendRecord)
		if err != nil {
			return nil, maskAny(err)
		}
		return result, nil
	}

	// Create ServiceRegistrations from raw ingresses
	var result ServiceRegistrations
	for _, rule := range i.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		host := rule.Host
		for _, httpPath := range rule.HTTP.Paths {
			instance := ServiceInstance{
				IP:   httpPath.Backend.ServiceName,
				Port: httpPath.Backend.ServicePort.IntValue(),
			}
			selector := ServiceSelector{
				Domain: host,
			}
			if strings.TrimPrefix(httpPath.Path, "/") != "" {
				selector.PathPrefix = httpPath.Path
			}
			sr := ServiceRegistration{
				ServiceName:     fmt.Sprintf("%s-%s", httpPath.Backend.ServiceName, hashOf(host, httpPath.Path, httpPath.Backend.ServiceName, httpPath.Backend.ServicePort.String())),
				ServicePort:     httpPath.Backend.ServicePort.IntValue(),
				Instances:       ServiceInstances{instance},
				Selectors:       ServiceSelectors{selector},
				EdgePort:        eb.config.PublicEdgePort,
				Public:          true,
				Mode:            "http",
				HttpCheckPath:   "",
				HttpCheckMethod: "",
				Sticky:          false,
				Backup:          false,
			}
			result = append(result, sr)
		}
	}
	return result, nil
}

// createServiceRegistrationsFromFrontendRecord creates ServiceRegistrations from the given FrontendRecord.
func (eb *k8sBackend) createServiceRegistrationsFromFrontendRecord(i k8s.Ingress, record api.FrontendRecord) (ServiceRegistrations, error) {
	var serviceMap map[string]struct{}
	var services []regapi.Service
	createService := func(backend k8s.IngressBackend) {
		key := fmt.Sprintf("%s-%s", backend.ServiceName, backend.ServicePort.String())
		if _, found := serviceMap[key]; !found {
			service := regapi.Service{
				ServiceName: backend.ServiceName,
				ServicePort: backend.ServicePort.IntValue(),
				Instances: []regapi.ServiceInstance{
					regapi.ServiceInstance{
						IP:   backend.ServiceName,
						Port: backend.ServicePort.IntValue(),
					},
				},
			}
			serviceMap[key] = struct{}{}
			services = append(services, service)
		}
	}

	if i.Spec.Backend != nil {
		createService(*i.Spec.Backend)
	}
	for _, rule := range i.Spec.Rules {
		if rule.HTTP != nil {
			for _, httpPath := range rule.HTTP.Paths {
				createService(httpPath.Backend)
			}
		}
	}

	result, err := mergeTrees(eb.Logger, eb.config, services, []api.FrontendRecord{record})
	if err != nil {
		return nil, maskAny(err)
	}
	return result, nil
}

// listIngresses returns all ingresses found in all namespaces.
func (eb *k8sBackend) listIngresses() ([]k8s.Ingress, error) {
	namespaces, err := eb.client.ListNamespaces(nil)
	if err != nil {
		return nil, maskAny(err)
	}
	var all []k8s.Ingress
	for _, ns := range namespaces.Items {
		list, err := eb.client.ListIngresses(ns.Name, nil)
		if err != nil {
			return nil, maskAny(err)
		}
		all = append(all, list.Items...)
	}
	return all, nil
}

func hashOf(s ...string) string {
	all := strings.Join(s, ",")
	return fmt.Sprintf("%x", sha1.Sum([]byte(all)))[:8]
}
