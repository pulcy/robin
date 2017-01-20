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
	RobinFrontendRecordsAnnotationKey = "pulcy.com.robin.frontend.records"
)

type k8sBackend struct {
	config         BackendConfig
	client         k8s.Client
	Logger         *logging.Logger
	services       ServiceRegistrations
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
		list, err := eb.createServices()
		if err != nil {
			return maskAny(err)
		}
		state := list.FullString()
		if state == eb.lastKnownState {
			eb.Logger.Debugf("Watch: state remains the same")
			time.Sleep(time.Second * 10)
		} else {
			eb.lastKnownState = state
			eb.services = list
			eb.Logger.Debugf("Watch: state has changed")
			return nil
		}
	}
}

// Load all registered services
func (eb *k8sBackend) Services() (ServiceRegistrations, error) {
	return eb.services, nil
}

// Load all registered services
func (eb *k8sBackend) createServices() (ServiceRegistrations, error) {
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
	raw, found := i.GetAnnotations()[RobinFrontendRecordsAnnotationKey]
	if found {
		var frontendRecords []api.FrontendRecord
		if err := json.Unmarshal([]byte(raw), &frontendRecords); err != nil {
			return nil, maskAny(err)
		}
		result, err := eb.createServiceRegistrationsFromFrontendRecords(i, frontendRecords)
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
			selector := ServiceSelector{
				Domain: host,
			}
			if strings.TrimPrefix(httpPath.Path, "/") != "" {
				selector.PathPrefix = httpPath.Path
			}
			sr := ServiceRegistration{
				ServiceName:     fmt.Sprintf("%s-%s-%s", i.GetNamespace(), httpPath.Backend.ServiceName, hashOf(host, httpPath.Path, httpPath.Backend.ServiceName, httpPath.Backend.ServicePort.String())),
				ServicePort:     httpPath.Backend.ServicePort.IntValue(),
				Selectors:       ServiceSelectors{selector},
				EdgePort:        eb.config.PublicEdgePort,
				Public:          true,
				Mode:            "http",
				HttpCheckPath:   "",
				HttpCheckMethod: "",
				Sticky:          false,
				Backup:          false,
			}
			ips, err := eb.listServicePodIPsByIngress(httpPath.Backend, i)
			if err != nil {
				return nil, maskAny(err)
			}
			for _, ip := range ips {
				sr.Instances = append(sr.Instances, ServiceInstance{
					IP:   ip,
					Port: httpPath.Backend.ServicePort.IntValue(),
				})
			}

			result = append(result, sr)
		}
	}
	return result, nil
}

// createServiceRegistrationsFromFrontendRecord creates ServiceRegistrations from the given FrontendRecord.
func (eb *k8sBackend) createServiceRegistrationsFromFrontendRecords(i k8s.Ingress, records []api.FrontendRecord) (ServiceRegistrations, error) {
	serviceMap := make(map[string]struct{})
	var services []regapi.Service
	createServiceFromBackend := func(backend k8s.IngressBackend) error {
		key := fmt.Sprintf("%s-%s-%s", i.GetNamespace(), backend.ServiceName, backend.ServicePort.String())
		if _, found := serviceMap[key]; !found {
			ips, err := eb.listServicePodIPsByIngress(backend, i)
			if err != nil {
				return maskAny(err)
			}
			service := regapi.Service{
				ServiceName: fmt.Sprintf("%s_%s", i.Namespace, backend.ServiceName),
				ServicePort: backend.ServicePort.IntValue(),
			}
			for _, ip := range ips {
				service.Instances = append(service.Instances, regapi.ServiceInstance{
					IP:   ip,
					Port: backend.ServicePort.IntValue(),
				})
			}
			serviceMap[key] = struct{}{}
			services = append(services, service)
		}
		return nil
	}

	if i.Spec.Backend != nil {
		if err := createServiceFromBackend(*i.Spec.Backend); err != nil {
			return nil, maskAny(err)
		}
	}
	for _, rule := range i.Spec.Rules {
		if rule.HTTP != nil {
			for _, httpPath := range rule.HTTP.Paths {
				if err := createServiceFromBackend(httpPath.Backend); err != nil {
					return nil, maskAny(err)
				}
			}
		}
	}

	createServiceFromRecord := func(record api.FrontendRecord) error {
		serviceName := record.Service
		namespace := i.Namespace
		parts := strings.SplitN(serviceName, ".", 2)
		if len(parts) == 2 {
			serviceName = parts[0]
			namespace = parts[1]
		}

		for _, sel := range record.Selectors {
			if sel.ServicePort == 0 {
				continue
			}
			key := fmt.Sprintf("%s-%s-%d", namespace, serviceName, sel.ServicePort)
			if _, found := serviceMap[key]; !found {
				ips, err := eb.listServicePodIPsByName(namespace, serviceName)
				if err != nil {
					return maskAny(err)
				}
				service := regapi.Service{
					ServiceName: fmt.Sprintf("%s_%s", namespace, serviceName),
					ServicePort: sel.ServicePort,
				}
				for _, ip := range ips {
					service.Instances = append(service.Instances, regapi.ServiceInstance{
						IP:   ip,
						Port: sel.ServicePort,
					})
				}
				serviceMap[key] = struct{}{}
				services = append(services, service)
			}
		}
		return nil
	}

	for _, r := range records {
		if err := createServiceFromRecord(r); err != nil {
			return nil, maskAny(err)
		}
	}

	// Patch serviceName of records
	// record.ServiceName can be:
	// - `name` -> A namespace will be prefixed (with a '_' separator)
	// - `name.namespace` -> The '.' will be replaced with a '_' and namespace will be put in front
	for x, r := range records {
		serviceName := r.Service
		parts := strings.SplitN(serviceName, ".", 2)
		if len(parts) == 1 {
			serviceName = i.Namespace + "_" + serviceName
		} else {
			serviceName = parts[1] + "_" + parts[0]
		}
		records[x].Service = serviceName
	}

	result, err := mergeTrees(eb.Logger, eb.config, services, records)
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

// listServicePodIPs returns the IP addresses of all pods that match the service name in the given ingress backend.
func (eb *k8sBackend) listServicePodIPsByIngress(backend k8s.IngressBackend, i k8s.Ingress) ([]string, error) {
	return eb.listServicePodIPsByName(i.GetNamespace(), backend.ServiceName)
}

// listServicePodIPs returns the IP addresses of all pods that match the service name in the given ingress backend.
func (eb *k8sBackend) listServicePodIPsByName(namespace, serviceName string) ([]string, error) {
	// Find service
	eb.Logger.Debugf("Fetching IPs for service '%s' in '%s'", serviceName, namespace)
	srv, err := eb.client.GetService(namespace, serviceName)
	if err != nil {
		// Service not yet known, just return an empty list
		eb.Logger.Warningf("Cannot find service '%s' in '%s'", serviceName, namespace)
		return nil, nil
	}
	selector := srv.Spec.Selector
	// Find matching pods
	pods, err := eb.client.ListPods(namespace, &k8s.ListOptions{
		LabelSelector: k8s.LabelSelector{
			MatchLabels: selector,
		},
	})
	if err != nil {
		eb.Logger.Warningf("Failed to list pods for service '%s' in '%s': %#v", serviceName, namespace, err)
		return nil, maskAny(err)
	}
	// Get IP's of pods
	result := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		status := p.Status
		if status != nil && status.Phase == k8s.PodRunning {
			result = append(result, status.PodIP)
		}
	}
	eb.Logger.Debugf("Found %d IP's for service '%s' in '%s'", len(result), serviceName, namespace)
	return result, nil
}

func hashOf(s ...string) string {
	all := strings.Join(s, ",")
	return fmt.Sprintf("%x", sha1.Sum([]byte(all)))[:8]
}

func createHostName(backend k8s.IngressBackend, i k8s.Ingress) string {
	return fmt.Sprintf("%s.%s", backend.ServiceName, i.GetNamespace())
}
