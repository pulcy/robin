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
	"fmt"

	logging "github.com/op/go-logging"
	regapi "github.com/pulcy/registrator-api"
	"github.com/pulcy/robin-api"
)

// mergeTrees merges the 2 trees into a single list of registrations.
func mergeTrees(log *logging.Logger, config BackendConfig, services []regapi.Service, frontends []api.FrontendRecord) (ServiceRegistrations, error) {
	result := ServiceRegistrations{}
	for _, s := range services {
		serviceName := s.ServiceName
		servicePort := s.ServicePort

		createServiceRegistration := func(edgePort int, public bool, mode string) *ServiceRegistration {
			service := &ServiceRegistration{
				ServiceName: serviceName,
				ServicePort: servicePort,
				EdgePort:    edgePort,
				Public:      public,
				Mode:        mode,
			}
			for _, si := range s.Instances {
				service.Instances = append(service.Instances, ServiceInstance{
					IP:   si.IP,
					Port: si.Port,
				})
			}
			log.Debugf("Created service '%s' edge-port=%d, public=%v, mode=%s", serviceName, edgePort, public, mode)
			return service
		}
		servicesByEdge := make(map[string]*ServiceRegistration)
		getServiceRegistration := func(edgePort int, private bool, mode string) *ServiceRegistration {
			if mode == "" {
				mode = "http"
			}
			if edgePort == 0 {
				if private {
					if mode == "http" {
						edgePort = config.PrivateHttpEdgePort
					} else {
						edgePort = config.PrivateTcpEdgePort
					}
				} else {
					edgePort = config.PublicEdgePort
				}
			}
			key := fmt.Sprintf("%d-%v", edgePort, private)
			sr, ok := servicesByEdge[key]
			if !ok {
				sr = createServiceRegistration(edgePort, !private, mode)
				servicesByEdge[key] = sr
			} else {
				if sr.Mode != mode {
					log.Errorf("Service %s has selectors with mode '%s' and mode '%s", sr.ServiceName, sr.Mode, mode)
				}
			}
			return sr
		}

		for _, fr := range frontends {
			frExtService := fmt.Sprintf("%s-%d", fr.Service, servicePort)
			if serviceName != fr.Service && serviceName != frExtService {
				continue
			}
			for _, sel := range fr.Selectors {
				if sel.ServicePort != 0 && sel.ServicePort != servicePort {
					continue
				}
				service := getServiceRegistration(sel.FrontendPort, sel.Private, fr.Mode)
				if fr.HttpCheckPath != "" && service.HttpCheckPath == "" {
					service.HttpCheckPath = fr.HttpCheckPath
				}
				if fr.HttpCheckMethod != "" && service.HttpCheckMethod == "" {
					service.HttpCheckMethod = fr.HttpCheckMethod
				}
				if fr.Sticky {
					service.Sticky = true
				}
				if fr.Backup {
					service.Backup = true
				}
				srSel := ServiceSelector{
					Weight:      sel.Weight,
					Domain:      sel.Domain,
					SslCertName: sel.SslCert,
					PathPrefix:  sel.PathPrefix,
				}
				for _, rwRule := range sel.RewriteRules {
					srSel.RewriteRules = append(srSel.RewriteRules, RewriteRule{
						PathPrefix:       rwRule.PathPrefix,
						RemovePathPrefix: rwRule.RemovePathPrefix,
						Domain:           rwRule.Domain,
					})
				}
				for _, user := range sel.Users {
					srSel.Users = append(srSel.Users, User{
						Name:         user.Name,
						PasswordHash: user.PasswordHash,
					})
				}
				if !service.Selectors.Contains(srSel) {
					log.Debugf("Selector %s added to service %s:%d", srSel.FullString(), serviceName, servicePort)
					service.Selectors = append(service.Selectors, srSel)
				} else {
					log.Debugf("Selector %s already found in service %s:%d", srSel.FullString(), serviceName, servicePort)
				}
			}
		}
		for _, service := range servicesByEdge {
			result = append(result, *service)
		}
	}
	return result, nil
}
