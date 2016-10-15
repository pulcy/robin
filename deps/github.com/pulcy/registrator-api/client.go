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

package api

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
	"golang.org/x/net/context"
)

const (
	DefaultEtcdPath      = "/pulcy/service"
	recentWatchErrorsMax = 5
)

type registratorClient struct {
	client            client.Client
	watcher           client.Watcher
	Logger            *logging.Logger
	prefix            string
	recentWatchErrors int
}

// NewRegistratorClient creates a new registrator API client from the given arguments.
// The etcdClient is required, all other arguments are options and will be set to default values if not given.
func NewRegistratorClient(etcdClient client.Client, etcdPath string, logger *logging.Logger) (API, error) {
	if etcdPath == "" {
		etcdPath = DefaultEtcdPath
	}
	if logger == nil {
		logger = logging.MustGetLogger("registrator-api")
	}
	return &registratorClient{
		client: etcdClient,
		prefix: etcdPath,
		Logger: logger,
	}, nil
}

// Watch for changes on a path and return where there is a change.
func (c *registratorClient) Watch() error {
	if c.watcher == nil || c.recentWatchErrors > recentWatchErrorsMax {
		c.recentWatchErrors = 0
		keyAPI := client.NewKeysAPI(c.client)
		options := &client.WatcherOptions{
			Recursive: true,
		}
		c.watcher = keyAPI.Watcher(c.prefix, options)
	}
	_, err := c.watcher.Next(context.Background())
	if err != nil {
		c.recentWatchErrors++
		return maskAny(err)
	}
	c.recentWatchErrors = 0
	return nil
}

// Load all registered services
func (c *registratorClient) Services() ([]Service, error) {
	keyAPI := client.NewKeysAPI(c.client)
	options := &client.GetOptions{
		Recursive: true,
		Sort:      false,
	}
	resp, err := keyAPI.Get(context.Background(), c.prefix, options)
	if err != nil {
		return nil, maskAny(err)
	}
	var list []Service
	if resp.Node == nil {
		return list, nil
	}
	for _, serviceNode := range resp.Node.Nodes {
		serviceName := path.Base(serviceNode.Key)
		partialServices := make(map[int]*Service)
		for _, instanceNode := range serviceNode.Nodes {
			uniqueID := path.Base(instanceNode.Key)
			parts := strings.Split(uniqueID, ":")
			if len(parts) < 3 {
				c.Logger.Warning("UniqueID malformed: '%s'", uniqueID)
				continue
			}
			port, err := strconv.Atoi(parts[2])
			if err != nil {
				c.Logger.Warning("Failed to parse port: '%s'", parts[2])
				continue
			}
			instance, err := c.parseServiceInstance(instanceNode.Value)
			if err != nil {
				c.Logger.Warning("Failed to parse instance '%s': %#v", instanceNode.Value, err)
				continue
			}
			s, ok := partialServices[port]
			if !ok {
				s = &Service{ServiceName: stripPortFromServiceName(serviceName, port), ServicePort: port}
				partialServices[port] = s
			}
			s.Instances = append(s.Instances, instance)

			// Register instance as separate service
			instanceName := parts[1]
			if strings.HasPrefix(instanceName, serviceName+"-") {
				s := Service{ServiceName: instanceName, ServicePort: port}
				s.Instances = append(s.Instances, instance)
				list = append(list, s)
			}
		}
		for _, v := range partialServices {
			list = append(list, *v)
		}
	}

	return list, nil
}

// parseServiceInstance parses a string in the format of "<ip>':'<port>" into a ServiceInstance.
func (c *registratorClient) parseServiceInstance(s string) (ServiceInstance, error) {
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

func stripPortFromServiceName(serviceName string, port int) string {
	suffix := fmt.Sprintf("-%d", port)
	return strings.TrimSuffix(serviceName, suffix)
}
