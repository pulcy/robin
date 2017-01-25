// Copyright (c) 2017 Pulcy.
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
	"sync"

	k8s "github.com/YakLabs/k8s-client"
	"github.com/YakLabs/k8s-client/http"
	logging "github.com/op/go-logging"
)

const (
	defaultWatchBufferSize = 32
)

func newResourceRegistry(log *logging.Logger) (*resourceRegistry, error) {
	client, err := http.NewInCluster()
	if err != nil {
		return nil, maskAny(err)
	}
	return &resourceRegistry{
		client:          client,
		log:             log,
		watchBufferSize: defaultWatchBufferSize,
		nodes:           make(map[string]k8s.Node),
		services:        make(map[string]k8s.Service),
		endpoints:       make(map[string]k8s.Endpoints),
		ingresses:       make(map[string]k8s.Ingress),
	}, nil
}

type resourceRegistry struct {
	client          k8s.Client
	log             *logging.Logger
	accessMutex     sync.RWMutex
	watchBufferSize int

	nodes     map[string]k8s.Node
	services  map[string]k8s.Service
	endpoints map[string]k8s.Endpoints
	ingresses map[string]k8s.Ingress
}

// Start runs watches on the apiserver and maintains the current state of the resources in it.
// It sends an event in the given channel when a change is detected.
func (r *resourceRegistry) Start(onChange chan struct{}) {
	// Watch nodes
	go func() {
		for {
			events := make(chan k8s.NodeWatchEvent, r.watchBufferSize)
			go func() {
				for evt := range events {
					if r.updateNode(evt) {
						onChange <- struct{}{}
					}
				}
			}()
			r.log.Debugf("watching node events")
			if err := r.client.WatchNodes(nil, events); err != nil {
				r.log.Errorf("WatchNodes failed: %v", err)
			}
		}
	}()

	// Watch ingresses
	go func() {
		for {
			events := make(chan k8s.IngressWatchEvent, r.watchBufferSize)
			go func() {
				for evt := range events {
					if r.updateIngress(evt) {
						onChange <- struct{}{}
					}
				}
			}()
			r.log.Debugf("watching ingress events")
			if err := r.client.WatchIngresses("", nil, events); err != nil {
				r.log.Errorf("WatchIngresses failed: %v", err)
			}
		}
	}()

	// Watch endpoints
	go func() {
		for {
			events := make(chan k8s.EndpointsWatchEvent, r.watchBufferSize)
			go func() {
				for evt := range events {
					if r.updateEndpoints(evt) {
						onChange <- struct{}{}
					}
				}
			}()
			r.log.Debugf("watching endpoints events")
			if err := r.client.WatchEndpoints("", nil, events); err != nil {
				r.log.Errorf("WatchEndpoints failed: %v", err)
			}
		}
	}()

	// Watch services
	go func() {
		for {
			events := make(chan k8s.ServiceWatchEvent, r.watchBufferSize)
			go func() {
				for evt := range events {
					if r.updateService(evt) {
						onChange <- struct{}{}
					}
				}
			}()
			r.log.Debugf("watching service events")
			if err := r.client.WatchServices("", nil, events); err != nil {
				r.log.Errorf("WatchServices failed: %v", err)
			}
		}
	}()
}

// GetNode returns a node by name.
func (r *resourceRegistry) GetNode(nodeName string) (k8s.Node, bool) {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	key := r.createKey("", nodeName)
	result, ok := r.nodes[key]
	return result, ok
}

// GetNodes returns a list of all known nodes
func (r *resourceRegistry) GetNodes() []k8s.Node {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	result := make([]k8s.Node, 0, len(r.nodes))
	for _, s := range r.nodes {
		result = append(result, s)
	}
	return result
}

// GetIngress returns an ingress by namespace+name.
func (r *resourceRegistry) GetIngress(namespace, ingressName string) (k8s.Ingress, bool) {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	key := r.createKey(namespace, ingressName)
	result, ok := r.ingresses[key]
	return result, ok
}

// GetIngresses returns a list of all known ingresses
func (r *resourceRegistry) GetIngresses() []k8s.Ingress {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	result := make([]k8s.Ingress, 0, len(r.ingresses))
	for _, s := range r.ingresses {
		result = append(result, s)
	}
	return result
}

// GetEndpoint returns an endpoint by namespace+name.
func (r *resourceRegistry) GetEndpoint(namespace, endpointsName string) (k8s.Endpoints, bool) {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	key := r.createKey(namespace, endpointsName)
	result, ok := r.endpoints[key]
	return result, ok
}

// GetEndpoints returns a list of all known endpoints
func (r *resourceRegistry) GetEndpoints() []k8s.Endpoints {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	result := make([]k8s.Endpoints, 0, len(r.endpoints))
	for _, s := range r.endpoints {
		result = append(result, s)
	}
	return result
}

// GetService returns a service by namespace+name.
func (r *resourceRegistry) GetService(namespace, serviceName string) (k8s.Service, bool) {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	key := r.createKey(namespace, serviceName)
	result, ok := r.services[key]
	return result, ok
}

// GetServices returns a list of all known services
func (r *resourceRegistry) GetServices() []k8s.Service {
	r.accessMutex.RLock()
	defer r.accessMutex.RUnlock()

	result := make([]k8s.Service, 0, len(r.services))
	for _, s := range r.services {
		result = append(result, s)
	}
	return result
}

func (r *resourceRegistry) updateNode(evt k8s.NodeWatchEvent) bool {
	switch evt.Type() {
	case k8s.WatchEventTypeModified:
		// We do not care about node modifications
		return false
	case k8s.WatchEventTypeAdded, k8s.WatchEventTypeDeleted:
		resource, err := evt.Object()
		if err != nil {
			r.log.Errorf("Failed to process resource event: %#v", err)
			return false
		}
		r.log.Debugf("Node %s %s", resource.Name, evt.Type())
		key := r.createKey(resource.Namespace, resource.Name)
		r.accessMutex.Lock()
		defer r.accessMutex.Unlock()
		if evt.Type() == k8s.WatchEventTypeDeleted {
			delete(r.nodes, key)
		} else {
			r.nodes[key] = *resource
		}
		return true
	default:
		r.log.Warningf("unknown node watch event of type '%s'", evt.Type())
		return false
	}
}

func (r *resourceRegistry) updateService(evt k8s.ServiceWatchEvent) bool {
	switch evt.Type() {
	case k8s.WatchEventTypeAdded, k8s.WatchEventTypeModified, k8s.WatchEventTypeDeleted:
		resource, err := evt.Object()
		if err != nil {
			r.log.Errorf("Failed to process resource event: %#v", err)
			return false
		}
		r.log.Debugf("Service %s.%s %s", resource.Name, resource.Namespace, evt.Type())
		key := r.createKey(resource.Namespace, resource.Name)
		r.accessMutex.Lock()
		defer r.accessMutex.Unlock()
		if evt.Type() == k8s.WatchEventTypeDeleted {
			delete(r.services, key)
		} else {
			r.services[key] = *resource
		}
		return true
	default:
		r.log.Warningf("unknown service watch event of type '%s'", evt.Type())
		return false
	}
}

func (r *resourceRegistry) updateEndpoints(evt k8s.EndpointsWatchEvent) bool {
	switch evt.Type() {
	case k8s.WatchEventTypeAdded, k8s.WatchEventTypeModified, k8s.WatchEventTypeDeleted:
		resource, err := evt.Object()
		if err != nil {
			r.log.Errorf("Failed to process resource event: %#v", err)
			return false
		}
		r.log.Debugf("Endpoints %s.%s %s", resource.Name, resource.Namespace, evt.Type())
		key := r.createKey(resource.Namespace, resource.Name)
		r.accessMutex.Lock()
		defer r.accessMutex.Unlock()
		if evt.Type() == k8s.WatchEventTypeDeleted {
			delete(r.endpoints, key)
			return true
		}
		// Check for different addresses
		existing, found := r.endpoints[key]
		if !found || endpointChanged(existing, *resource) {
			r.endpoints[key] = *resource
			return true
		}
		return false
	default:
		r.log.Warningf("unknown endpoints watch event of type '%s'", evt.Type())
		return false
	}
}

func endpointChanged(a, b k8s.Endpoints) bool {
	describe := func(x k8s.Endpoints) string {
		result := ""
		for _, subset := range x.Subsets {
			result = result + "{"
			for _, addr := range subset.Addresses {
				result = result + addr.IP + " "
			}
			result = result + "/"
			for _, addr := range subset.NotReadyAddresses {
				result = result + addr.IP + " "
			}
			result = result + "}"
		}
		return result
	}
	return describe(a) != describe(b)
}

func (r *resourceRegistry) updateIngress(evt k8s.IngressWatchEvent) bool {
	switch evt.Type() {
	case k8s.WatchEventTypeAdded, k8s.WatchEventTypeModified, k8s.WatchEventTypeDeleted:
		resource, err := evt.Object()
		if err != nil {
			r.log.Errorf("Failed to process resource event: %#v", err)
			return false
		}
		r.log.Debugf("Ingress %s.%s %s", resource.Name, resource.Namespace, evt.Type())
		key := r.createKey(resource.Namespace, resource.Name)
		r.accessMutex.Lock()
		defer r.accessMutex.Unlock()
		if evt.Type() == k8s.WatchEventTypeDeleted {
			delete(r.ingresses, key)
		} else {
			r.ingresses[key] = *resource
		}
		return true
	default:
		r.log.Warningf("unknown ingress watch event of type '%s'", evt.Type())
		return false
	}
}

func (r *resourceRegistry) createKey(namespace, resourceName string) string {
	return resourceName + "." + namespace
}
