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

import (
	"fmt"

	"github.com/xenolf/lego/acme"

	"github.com/pulcy/robin/service/backend"
)

const (
	acmeServiceName = "__acme"
	acmeServicePort = 0
)

type AcmeServiceListener interface {
	CertificatesUpdated() // Called when there is a change in one of the ACME generated certificates.
}

type AcmeServiceConfig struct {
	HttpProviderConfig

	EtcdPrefix       string // Folder in ETCD to use ACME
	CADirectoryURL   string // URL of ACME directory
	KeyBits          int    // Size of generated keys (in bits)
	Email            string // Registration email address
	PrivateKeyPath   string // Path of file containing private key
	RegistrationPath string // Path of file containing acme.RegistrationResource
}

type AcmeServiceDependencies struct {
	HttpProviderDependencies

	Listener   AcmeServiceListener
	Repository CertificatesRepository
	Cache      CertificatesFileCache
	Renewal    RenewalMonitor
	Requester  CertificateRequester
}

type AcmeService interface {
	Register() error
	Start() error
	Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error)
}

type acmeService struct {
	AcmeServiceConfig
	AcmeServiceDependencies

	httpProvider *httpChallengeProvider
	active       bool
}

// NewAcmeService creates and initializes a new AcmeService implementation.
func NewAcmeService(config AcmeServiceConfig, deps AcmeServiceDependencies) AcmeService {
	return &acmeService{
		AcmeServiceConfig:       config,
		AcmeServiceDependencies: deps,

		httpProvider: newHttpChallengeProvider(config.HttpProviderConfig, deps.HttpProviderDependencies),
	}
}

// Start launches this services.
func (s *acmeService) Start() error {
	// Check arguments
	missingArgs := []string{}
	if s.Email == "" {
		missingArgs = append(missingArgs, "acme-email")
	}
	if s.CADirectoryURL == "" {
		missingArgs = append(missingArgs, "acme-directory-url")
	}
	if s.PrivateKeyPath == "" {
		missingArgs = append(missingArgs, "private-key-path")
	}
	if s.RegistrationPath == "" {
		missingArgs = append(missingArgs, "registration-path")
	}

	if len(missingArgs) > 0 {
		s.Logger.Warning("ACME is not configured, some it will not be used. Missing: %v", missingArgs)
		return nil
	}

	// Load private key
	key, err := s.getPrivateKey()
	if err != nil {
		return maskAny(err)
	}

	// Load registration
	registration, err := s.getRegistration()
	if err != nil {
		return maskAny(err)
	}
	if registration == nil {
		return maskAny(fmt.Errorf("No registration found at %s", s.RegistrationPath))
	}

	// Create ACME client
	user := acmeUser{
		Email:        s.Email,
		Registration: registration,
		PrivateKey:   key,
	}
	client, err := acme.NewClient(s.CADirectoryURL, user, s.KeyBits)
	if err != nil {
		return maskAny(err)
	}
	client.ExcludeChallenges([]acme.Challenge{acme.TLSSNI01, acme.DNS01})
	client.SetChallengeProvider(acme.HTTP01, newHttpChallengeProvider(s.HttpProviderConfig, s.HttpProviderDependencies))

	// Save objects
	s.Requester.Initialize(client)

	// Start HTTP challenge listener
	if err := s.httpProvider.Start(); err != nil {
		return maskAny(err)
	}

	// Monitor the repository for changes
	s.repositoryMonitorLoop()

	// Start the renewal monitor
	s.Renewal.Start()

	// We're now active
	s.active = true

	return nil
}

// repositoryMonitorLoop monitors the certificates repository and flushes the
// domain file cache when there is a change in the repository.
func (s *acmeService) repositoryMonitorLoop() {
	go func() {
		for {
			s.Cache.Clear()
			s.Listener.CertificatesUpdated()
			s.Repository.WatchDomainCertificates()
		}
	}()
}

// Extend fills is missing data provided by ACME into the list of services.
// It also adds a service to handle ACME HTTP challenges
func (s *acmeService) Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error) {
	if !s.active {
		// Not active, so nothing to extend
		return services, nil
	}

	// Find domains that need a certificate
	domainSet := make(map[string]struct{})
	domains := []string{}
	allDomains := []string{}
	updatedServices := backend.ServiceRegistrations{}
	for _, sr := range services {
		for selIndex, sel := range sr.Selectors {
			if sel.Private || sel.SslCertName != "" || sel.Domain == "" {
				continue
			}
			// Domain needs a certificate, try cache first
			domain := sel.Domain
			allDomains = append(allDomains, domain)
			path, err := s.Cache.GetDomainCertificatePath(domain)
			if err != nil {
				s.Logger.Error("Failed to get domain certificate path for '%s': %#v", domain, err)
			} else if path != "" {
				// Certificate path found
				sr.Selectors[selIndex].TmpSslCertPath = path
			} else {
				// We need to request a certificate
				if _, ok := domainSet[domain]; !ok {
					domainSet[domain] = struct{}{}
					domains = append(domains, domain)
				}
			}
		}
		updatedServices = append(updatedServices, sr)
	}

	// Request certificates for the domains
	if len(domains) > 0 {
		go func() {
			// Now request the certificates
			if err := s.Requester.RequestCertificates(domains); err != nil {
				if IsNotMaster(err) {
					s.Logger.Info("Another instance is master, so requesting certificates is cancelled.")
				} else {
					s.Logger.Error("Failed to request certificates: %#v", err)
				}
			}
		}()
	}

	// Add HTTP challenge service
	updatedServices = append(updatedServices, s.createAcmeServiceRegistration())

	// Inform the renewal monitor
	s.Renewal.SetUsedDomains(allDomains)

	return updatedServices, nil
}

// createAcmeServiceRegistration creates a ServiceRegistration item for the ACME HTTP challenge
func (s *acmeService) createAcmeServiceRegistration() backend.ServiceRegistration {
	pathPrefix := acme.HTTP01ChallengePath("")
	sr := backend.ServiceRegistration{
		ServiceName: acmeServiceName,
		ServicePort: acmeServicePort,
		Instances: backend.ServiceInstances{
			backend.ServiceInstance{
				IP:   "127.0.0.1",
				Port: s.HttpProviderConfig.Port,
			},
		},
		Selectors: backend.ServiceSelectors{
			backend.ServiceSelector{
				Weight:     100,
				PathPrefix: pathPrefix,
				Private:    false,
			},
		},
		HttpCheckPath: "",
	}
	return sr
}
