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

package main

import (
	"github.com/mitchellh/go-homedir"
	"github.com/pulcy/robin/service"
	"github.com/pulcy/robin/service/backend"
)

const (
	projectName = "robin"
)

const (
	defaultStatsPort         = 7088
	defaultStatsSslCert      = ""
	defaultSslCertsFolder    = "/certs/"
	defaultForceSsl          = false
	defaultPrivateHost       = ""
	defaultPublicHost        = ""
	defaultPrivateTcpSslCert = ""
	defaultLogLevel          = "info"
)

const (
	defaultAcmeHttpPort         = 8011
	defaultKeyBits              = 4096
	defaultCADirectoryURL       = "https://acme-v01.api.letsencrypt.org/directory"
	defaultPrivateKeyPathTmpl   = "~/.pulcy/acme/private-key.pem"
	defaultRegistrationPathTmpl = "~/.pulcy/acme/registration.json"
	defaultTmpCertificatePath   = "/tmp/certificates"
)

const (
	defaultMetricsHost      = "0.0.0.0"
	defaultMetricsPort      = 8055
	defaultPrivateStatsPort = 7089
)

const (
	defaultApiHost = "0.0.0.0"
	defaultApiPort = 8056
)

var (
	etcdBackendConfig = backend.BackendConfig{
		PublicEdgePort:      service.PublicHttpPort,
		PrivateHttpEdgePort: service.PrivateHttpPort,
		PrivateTcpEdgePort:  service.PrivateTcpSslPort,
	}
)

func defaultPrivateKeyPath() string {
	result, err := homedir.Expand(defaultPrivateKeyPathTmpl)
	if err != nil {
		Exitf("Cannot expand private-key-path: %#v", err)
	}
	return result
}

func defaultRegistrationPath() string {
	result, err := homedir.Expand(defaultRegistrationPathTmpl)
	if err != nil {
		Exitf("Cannot expand registration-path: %#v", err)
	}
	return result
}
