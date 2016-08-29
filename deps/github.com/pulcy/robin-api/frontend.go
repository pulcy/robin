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

type FrontendRecord struct {
	Selectors       []FrontendSelectorRecord `json:"selectors"`
	Service         string                   `json:"service,omitempty"`
	Mode            string                   `json:"mode,omitempty"` // http|tcp
	HttpCheckPath   string                   `json:"http-check-path,omitempty"`
	HttpCheckMethod string                   `json:"http-check-method,omitempty"`
	Sticky          bool                     `json:"sticky,omitempty"`
	Backup          bool                     `json:"backup,omitempty"`
}

type FrontendSelectorRecord struct {
	Weight       int           `json:"weight,omitempty"`
	Domain       string        `json:"domain,omitempty"`
	PathPrefix   string        `json:"path-prefix,omitempty"`
	SslCert      string        `json:"ssl-cert,omitempty"`
	Port         int           `json:"port,omitempty"`
	Private      bool          `json:"private,omitempty"`
	Users        []UserRecord  `json:"users,omitempty"`
	RewriteRules []RewriteRule `json:"rewrite-rules,omitempty"`
}

type UserRecord struct {
	Name         string `json:"user"`
	PasswordHash string `json:"pwhash"`
}

type RewriteRule struct {
	PathPrefix       string `json:"path-prefix,omitempty"`        // Add this to the start of the request path.
	RemovePathPrefix string `json:"remove-path-prefix,omitempty"` // Remove this from the start of the request path.
	Domain           string `json:"domain,omitempty"`             // Redirect to this domain
}
