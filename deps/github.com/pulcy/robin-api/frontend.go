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

import "github.com/juju/errgo"

const (
	maxPort = 64 * 1024
)

type FrontendRecord struct {
	Selectors       []FrontendSelectorRecord `json:"selectors"`
	Service         string                   `json:"service,omitempty"`
	Mode            string                   `json:"mode,omitempty"` // http|tcp
	HttpCheckPath   string                   `json:"http-check-path,omitempty"`
	HttpCheckMethod string                   `json:"http-check-method,omitempty"`
	Sticky          bool                     `json:"sticky,omitempty"`
	Backup          bool                     `json:"backup,omitempty"`
}

// Validate checks the given object for invalid values.
func (r FrontendRecord) Validate() error {
	if r.Service == "" {
		return maskAny(errgo.WithCausef(nil, ValidationError, "service must be set"))
	}
	switch r.Mode {
	case "", "http", "tcp":
	// OK
	default:
		return maskAny(errgo.WithCausef(nil, ValidationError, "mode must be http|tcp"))
	}
	if len(r.Selectors) == 0 {
		return maskAny(errgo.WithCausef(nil, ValidationError, "at least 1 selector must be set"))
	}
	for _, sr := range r.Selectors {
		if err := sr.Validate(); err != nil {
			return maskAny(err)
		}
	}
	return nil
}

type FrontendSelectorRecord struct {
	Weight       int           `json:"weight,omitempty"`
	Domain       string        `json:"domain,omitempty"`
	PathPrefix   string        `json:"path-prefix,omitempty"`
	SslCert      string        `json:"ssl-cert,omitempty"`
	ServicePort  int           `json:"port,omitempty"`
	FrontendPort int           `json:"frontend-port,omitempty"`
	Private      bool          `json:"private,omitempty"`
	Users        []UserRecord  `json:"users,omitempty"`
	RewriteRules []RewriteRule `json:"rewrite-rules,omitempty"`
}

// Validate checks the given object for invalid values.
func (r FrontendSelectorRecord) Validate() error {
	if r.Weight < 0 || r.Weight > 100 {
		return maskAny(errgo.WithCausef(nil, ValidationError, "weight must be between 0-100"))
	}
	if r.ServicePort < 0 || r.ServicePort > maxPort {
		return maskAny(errgo.WithCausef(nil, ValidationError, "port must be between 0-%d", maxPort))
	}
	if r.FrontendPort < 0 || r.FrontendPort > maxPort {
		return maskAny(errgo.WithCausef(nil, ValidationError, "frontend-port must be between 0-%d", maxPort))
	}
	if r.Domain == "" && r.PathPrefix == "" && r.FrontendPort == 0 {
		return maskAny(errgo.WithCausef(nil, ValidationError, "domain, path-prefix or frontend-port must be set"))
	}
	for _, ur := range r.Users {
		if err := ur.Validate(); err != nil {
			return maskAny(err)
		}
	}
	for _, rr := range r.RewriteRules {
		if err := rr.Validate(); err != nil {
			return maskAny(err)
		}
	}
	return nil
}

type UserRecord struct {
	Name         string `json:"user"`
	PasswordHash string `json:"pwhash"`
}

// Validate checks the given object for invalid values.
func (r UserRecord) Validate() error {
	if r.Name == "" {
		return maskAny(errgo.WithCausef(nil, ValidationError, "name must be set"))
	}
	if r.PasswordHash == "" {
		return maskAny(errgo.WithCausef(nil, ValidationError, "pwhash must be set"))
	}
	return nil
}

type RewriteRule struct {
	PathPrefix       string `json:"path-prefix,omitempty"`        // Add this to the start of the request path.
	RemovePathPrefix string `json:"remove-path-prefix,omitempty"` // Remove this from the start of the request path.
	Domain           string `json:"domain,omitempty"`             // Redirect to this domain
}

// Validate checks the given object for invalid values.
func (r RewriteRule) Validate() error {
	if r.PathPrefix == "" && r.RemovePathPrefix == "" && r.Domain == "" {
		return maskAny(errgo.WithCausef(nil, ValidationError, "at least 1 property must be set"))
	}
	if r.PathPrefix != "" && r.RemovePathPrefix != "" {
		return maskAny(errgo.WithCausef(nil, ValidationError, "path-prefix and remove-path-prefix cannot be set both"))
	}
	return nil
}
