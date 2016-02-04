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

package acme

type CertificatesRepository interface {
	WatchDomainCertificates() error

	// loadDomainCertificate tries to load the certificate for the given domain from the ETCD repository
	// Returns nil,nil if domain is not found.
	LoadDomainCertificate(domain string) ([]byte, error)

	// storeDomainCertificate stores the certificate for the given domain in the ETCD repository
	StoreDomainCertificate(domain string, certificate []byte) error
}
