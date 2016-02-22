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
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
)

const (
	servicePrefix  = "service"
	frontEndPrefix = "frontend"
)

type etcdBackend struct {
	client  client.Client
	watcher client.Watcher
	Logger  *logging.Logger
	prefix  string
}

func NewEtcdBackend(logger *logging.Logger, uri *url.URL) (Backend, error) {
	cfg := client.Config{
		Transport: client.DefaultTransport,
	}
	if uri.Host != "" {
		cfg.Endpoints = append(cfg.Endpoints, "http://"+uri.Host)
	}
	c, err := client.New(cfg)
	if err != nil {
		return nil, maskAny(err)
	}
	kAPI := client.NewKeysAPI(c)
	options := &client.WatcherOptions{
		Recursive: true,
	}
	watcher := kAPI.Watcher(uri.Path, options)
	return &etcdBackend{
		client:  c,
		watcher: watcher,
		prefix:  uri.Path,
		Logger:  logger,
	}, nil
}

// Watch for changes on a path and return where there is a change.
func (eb *etcdBackend) Watch() error {
	_, err := eb.watcher.Next(context.Background())
	if err != nil {
		return maskAny(err)
	}
	return nil
}

// Load all registered services
func (eb *etcdBackend) Services() (ServiceRegistrations, error) {
	servicesTree, err := eb.readServicesTree()
	if err != nil {
		return nil, maskAny(err)
	}
	frontEndTree, err := eb.readFrontEndsTree()
	if err != nil {
		return nil, maskAny(err)
	}
	result, err := eb.mergeTrees(servicesTree, frontEndTree)
	if err != nil {
		return nil, maskAny(err)
	}
	return result, nil
}

// Load all registered services
func (eb *etcdBackend) readServicesTree() (ServiceRegistrations, error) {
	etcdPath := path.Join(eb.prefix, servicePrefix)
	kAPI := client.NewKeysAPI(eb.client)
	options := &client.GetOptions{
		Recursive: true,
		Sort:      false,
	}
	resp, err := kAPI.Get(context.Background(), etcdPath, options)
	if err != nil {
		return nil, maskAny(err)
	}
	list := ServiceRegistrations{}
	if resp.Node == nil {
		return list, nil
	}
	for _, serviceNode := range resp.Node.Nodes {
		serviceName := path.Base(serviceNode.Key)
		registrations := make(map[int]*ServiceRegistration)
		for _, instanceNode := range serviceNode.Nodes {
			uniqueID := path.Base(instanceNode.Key)
			parts := strings.Split(uniqueID, ":")
			if len(parts) < 3 {
				eb.Logger.Warning("UniqueID malformed: '%s'", uniqueID)
				continue
			}
			port, err := strconv.Atoi(parts[2])
			if err != nil {
				eb.Logger.Warning("Failed to parse port: '%s'", parts[2])
				continue
			}
			instance, err := eb.parseServiceInstance(instanceNode.Value)
			if err != nil {
				eb.Logger.Warning("Failed to parse instance '%s': %#v", instanceNode.Value, err)
				continue
			}
			sr, ok := registrations[port]
			if !ok {
				sr = &ServiceRegistration{ServiceName: serviceName, ServicePort: port}
				registrations[port] = sr
			}
			sr.Instances = append(sr.Instances, instance)

			// Register instance as separate service
			instanceName := parts[1]
			if strings.HasPrefix(instanceName, serviceName+"-") {
				sr := ServiceRegistration{ServiceName: instanceName, ServicePort: port}
				sr.Instances = append(sr.Instances, instance)
				list = append(list, sr)
			}
		}
		for _, v := range registrations {
			list = append(list, *v)
		}
	}

	return list, nil
}

// parseServiceInstance parses a string in the format of "<ip>':'<port>" into a ServiceInstance.
func (eb *etcdBackend) parseServiceInstance(s string) (ServiceInstance, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return ServiceInstance{}, maskAny(fmt.Errorf("Invalid service instance '%s'", s))
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return ServiceInstance{}, maskAny(fmt.Errorf("Invalid service instance port '%s' in '%s'", parts[1], s))
	}
	return ServiceInstance{
		IP:   parts[0],
		Port: port,
	}, nil
}

type frontendRecord struct {
	Selectors     []frontendSelectorRecord `json:"selectors"`
	Service       string                   `json:"service,omitempty"`
	Mode          string                   `json:"mode,omitempty"` // http|tcp
	HttpCheckPath string                   `json:"http-check-path,omitempty"`
}

type frontendSelectorRecord struct {
	Weight     int          `json:"weight,omitempty"`
	Domain     string       `json:"domain,omitempty"`
	SslCert    string       `json:"ssl-cert,omitempty"`
	PathPrefix string       `json:"path-prefix,omitempty"`
	Port       int          `json:"port,omitempty"`
	Private    bool         `json:"private,omitempty"`
	Users      []userRecord `json:"users,omitempty"`
}

type userRecord struct {
	Name         string `json:"user"`
	PasswordHash string `json:"pwhash"`
}

// Load all registered front-ends
func (eb *etcdBackend) readFrontEndsTree() ([]frontendRecord, error) {
	etcdPath := path.Join(eb.prefix, frontEndPrefix)
	kAPI := client.NewKeysAPI(eb.client)
	options := &client.GetOptions{
		Recursive: false,
		Sort:      false,
	}
	resp, err := kAPI.Get(context.Background(), etcdPath, options)
	if err != nil {
		return nil, maskAny(err)
	}
	list := []frontendRecord{}
	if resp.Node == nil {
		return list, nil
	}
	for _, frontEndNode := range resp.Node.Nodes {
		rawJson := frontEndNode.Value
		record := frontendRecord{}
		if err := json.Unmarshal([]byte(rawJson), &record); err != nil {
			eb.Logger.Error("Cannot unmarshal registration of %s", frontEndNode.Key)
			continue
		}
		list = append(list, record)
	}

	return list, nil
}

// mergeTrees merges the 2 trees into a single list of registrations.
func (eb *etcdBackend) mergeTrees(services ServiceRegistrations, frontends []frontendRecord) (ServiceRegistrations, error) {
	result := ServiceRegistrations{}
	for _, service := range services {
		for _, fr := range frontends {
			if service.ServiceName != fr.Service {
				continue
			}
			if fr.HttpCheckPath != "" && service.HttpCheckPath == "" {
				service.HttpCheckPath = fr.HttpCheckPath
			}
			if fr.Mode != "" && service.Mode == "" {
				service.Mode = fr.Mode
			}
			for _, sel := range fr.Selectors {
				if sel.Port != 0 && sel.Port != service.ServicePort {
					continue
				}
				srSel := ServiceSelector{
					Weight:      sel.Weight,
					Domain:      sel.Domain,
					SslCertName: sel.SslCert,
					PathPrefix:  sel.PathPrefix,
					Private:     sel.Private,
				}
				for _, user := range sel.Users {
					srSel.Users = append(srSel.Users, User{
						Name:         user.Name,
						PasswordHash: user.PasswordHash,
					})
				}
				if !service.Selectors.Contains(srSel) {
					service.Selectors = append(service.Selectors, srSel)
				}
			}
		}
		if len(service.Selectors) > 0 {
			if service.Mode == "" {
				service.Mode = "http"
			}
			result = append(result, service)
		}
	}
	return result, nil
}
