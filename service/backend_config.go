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

package service

import (
	"fmt"

	"github.com/pulcy/robin/service/backend"
)

type backendConfig struct {
	Name     string
	Services backend.ServiceRegistrations
}

func (b backendConfig) IsSticky() (bool, error) {
	if len(b.Services) == 0 {
		return false, nil
	}
	result := b.Services[0].Sticky
	for _, sr := range b.Services {
		if sr.Sticky != result {
			return result, maskAny(fmt.Errorf("Conflicting sticky settings in backend %s", b.Name))
		}
	}
	return result, nil
}

func (b backendConfig) Mode() (string, error) {
	normalize := func(s string) string {
		if s == "" {
			return "http"
		}
		return s
	}
	if len(b.Services) == 0 {
		return normalize(""), nil
	}
	result := normalize(b.Services[0].Mode)
	for _, sr := range b.Services {
		mode := normalize(sr.Mode)
		if mode != result {
			return result, maskAny(fmt.Errorf("Conflicting mode settings in backend %s", b.Name))
		}
	}
	return result, nil
}

func (b backendConfig) HttpCheckPath() (string, bool, error) {
	services := b.httpCheckServices()
	normalize := func(s string) string {
		if s == "" {
			return "/"
		}
		return s
	}
	if len(services) == 0 {
		return normalize(""), false, nil
	}
	result := normalize(services[0].HttpCheckPath)
	for _, sr := range services {
		x := normalize(sr.HttpCheckPath)
		if x != result {
			return result, true, maskAny(fmt.Errorf("Conflicting HttpCheckPath settings in backend %s", b.Name))
		}
	}
	return result, true, nil
}

func (b backendConfig) HttpCheckMethod() (string, bool, error) {
	services := b.httpCheckServices()
	normalize := func(s string) string {
		if s == "" {
			return "GET"
		}
		return s
	}
	if len(services) == 0 {
		return normalize(""), false, nil
	}
	result := normalize(services[0].HttpCheckMethod)
	for _, sr := range services {
		x := normalize(sr.HttpCheckMethod)
		if x != result {
			return result, true, maskAny(fmt.Errorf("Conflicting HttpCheckMethod settings in backend %s", b.Name))
		}
	}
	return result, true, nil
}

func (b backendConfig) httpCheckServices() backend.ServiceRegistrations {
	var result backend.ServiceRegistrations
	for _, sr := range b.Services {
		if sr.HttpCheckPath != "" || sr.HttpCheckMethod != "" {
			result = append(result, sr)
		}
	}
	return result
}

func (b backendConfig) HasAllowUnauthorized() bool {
	for _, sr := range b.Services {
		if sr.HasAllowUnauthorized() {
			return true
		}
	}
	return false
}
