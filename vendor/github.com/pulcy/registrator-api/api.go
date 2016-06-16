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

type API interface {
	// Watch for changes in the services tree and return where there is a change.
	Watch() error

	// Load all registered services
	Services() ([]Service, error)
}

type Service struct {
	ServiceName string // Name of the service
	ServicePort int    // Port the service is listening on (inside its container)
	Instances   []ServiceInstance
}

type ServiceInstance struct {
	IP   string // IP address to connect to to reach the service instance
	Port int    // Port to connect to to reach the service instance
}
