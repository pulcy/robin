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
	"sync"

	k8s "github.com/YakLabs/k8s-client"
	"github.com/op/go-logging"
	regapi "github.com/pulcy/registrator-api"
	api "github.com/pulcy/robin-api"
)

const (
	RobinFrontendRecordsAnnotationKey = "pulcy.com.robin.frontend.records"
)

type k8sBackend struct {
	config        BackendConfig
	registry      *resourceRegistry
	Logger        *logging.Logger
	startRegistry sync.Once
	watchCond     *sync.Cond
}

func NewKubernetesBackend(config BackendConfig, logger *logging.Logger) (Backend, error) {
	registry, err := newResourceRegistry(logger)
	if err != nil {
		return nil, maskAny(err)
	}
	return &k8sBackend{
		config:    config,
		registry:  registry,
		Logger:    logger,
		watchCond: sync.NewCond(new(sync.Mutex)),
	}, nil
}

// Watch for changes on a path and return where there is a change.
func (eb *k8sBackend) Watch() error {
	eb.startRegistry.Do(func() { eb.start() })

	// Wait for events from the registry
	eb.watchCond.L.Lock()
	eb.watchCond.Wait()
	eb.watchCond.L.Unlock()
	return nil
}

// start starts the resource registry and listens for its changes.
func (eb *k8sBackend) start() {
	onChange := make(chan struct{})
	go func() {
		for _ = range onChange {
			// trigger Watch return
			eb.watchCond.Broadcast()
		}
	}()
	eb.registry.Start(onChange)
}

// Load all registered services
func (eb *k8sBackend) Services() (ServiceRegistrations, error) {
	ingresses := eb.registry.GetIngresses()
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
			ips, _, err := eb.listServicePodIPsByIngress(httpPath.Backend, i)
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
			ips, _, err := eb.listServicePodIPsByIngress(backend, i)
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
				activeIPs, notActiveIPs, err := eb.listServicePodIPsByName(namespace, serviceName)
				if err != nil {
					return maskAny(err)
				}
				service := regapi.Service{
					ServiceName: fmt.Sprintf("%s_%s", namespace, serviceName),
					ServicePort: sel.ServicePort,
				}
				for _, ip := range activeIPs {
					service.Instances = append(service.Instances, regapi.ServiceInstance{
						IP:   ip,
						Port: sel.ServicePort,
					})
				}
				if record.HttpCheckMethod != "" || record.HttpCheckPath != "" {
					for _, ip := range notActiveIPs {
						service.Instances = append(service.Instances, regapi.ServiceInstance{
							IP:   ip,
							Port: sel.ServicePort,
						})
					}
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

// listServicePodIPs returns the IP addresses of all pods that match the service name in the given ingress backend.
func (eb *k8sBackend) listServicePodIPsByIngress(backend k8s.IngressBackend, i k8s.Ingress) ([]string, []string, error) {
	return eb.listServicePodIPsByName(i.GetNamespace(), backend.ServiceName)
}

// listServicePodIPs returns the IP addresses of all endpoints that match the service name in the given ingress backend.
// The first set of IP addresses is from all active pods, the second set is from all not-yet-active pods.
func (eb *k8sBackend) listServicePodIPsByName(namespace, serviceName string) ([]string, []string, error) {
	eb.Logger.Debugf("searching for endpoints for service '%s' in '%s'", serviceName, namespace)
	// Find matching endpoints
	endpoints, found := eb.registry.GetEndpoint(namespace, serviceName)
	if !found {
		eb.Logger.Debugf("cannot find endpoints for service '%s' in '%s'", serviceName, namespace)
		return nil, nil, nil
	}
	// Get IP's
	var active []string
	var notYetActive []string
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			active = append(active, addr.IP)
		}
		for _, addr := range subset.NotReadyAddresses {
			notYetActive = append(notYetActive, addr.IP)
		}
	}
	eb.Logger.Debugf("found %d/%d IP's for service '%s' in '%s'", len(active), len(notYetActive), serviceName, namespace)
	return active, notYetActive, nil
}

func hashOf(s ...string) string {
	all := strings.Join(s, ",")
	return fmt.Sprintf("%x", sha1.Sum([]byte(all)))[:8]
}

func createHostName(backend k8s.IngressBackend, i k8s.Ingress) string {
	return fmt.Sprintf("%s.%s", backend.ServiceName, i.GetNamespace())
}
