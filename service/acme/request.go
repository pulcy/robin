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
	"time"

	"github.com/op/go-logging"
	"github.com/xenolf/lego/acme"

	"git.pulcy.com/pulcy/load-balancer/service/mutex"
)

const (
	requestCertificatesLockName = "requestCertificates"
	requestCertificatesLockTTL  = 30 // sec
	requestDelay                = time.Second * 5
)

type CertificateRequester interface {
	Initialize(acmeClient *acme.Client)
	RequestCertificates(domains []string) error
}

type certificateRequester struct {
	Logger       *logging.Logger
	Repository   CertificatesRepository
	mutexService mutex.GlobalMutexService

	acmeClient *acme.Client
}

func NewCertificateRequester(logger *logging.Logger, repository CertificatesRepository, mutexService mutex.GlobalMutexService) CertificateRequester {
	return &certificateRequester{
		Logger:       logger,
		Repository:   repository,
		mutexService: mutexService,
	}
}

func (cr *certificateRequester) Initialize(acmeClient *acme.Client) {
	cr.acmeClient = acmeClient
}

// requestCertificates tries to request certificates for all given domains.
// It first tries to claims to be the master. If that does not succeed,
// it returns a NotMasterError
func (s *certificateRequester) RequestCertificates(domains []string) error {
	isMaster, lock, err := s.claimRequestCertificatesMutex()
	if err != nil {
		return maskAny(err)
	}
	if !isMaster {
		s.Logger.Debug("requestCertificates ends because another instance is requesting certificates")
		return maskAny(NotMasterError)
	}

	// We're the master, let's request some certificates
	defer lock.Unlock()

	// Wait a bit to give haproxy the time to restart
	time.Sleep(requestDelay)

	failedDomains := []string{}
	for _, domain := range domains {
		s.Logger.Debug("Obtaining certificate for '%s'", domain)
		bundle := true
		certificates, failures := s.acmeClient.ObtainCertificate([]string{domain}, bundle, nil)
		if len(failures) > 0 {
			failedDomains = append(failedDomains, domain)
			s.Logger.Error("ObtainCertificate for '%s' failed: %#v", domain, failures)
			continue
		}

		// Store the domain so all instances can use it
		if err := s.saveCertificate(domain, certificates); err != nil {
			s.Logger.Error("Failed to save certificate for '%s': %#v", domain, err)
		} else {
			s.Logger.Info("Stored certificate for '%s' in repository", domain)
		}
	}

	if len(failedDomains) > 0 {
		return maskAny(fmt.Errorf("Failed to obtain certificates for %#v", failedDomains))
	}
	return nil
}

// saveCertificate stores the given certificate in ETCD.
func (s *certificateRequester) saveCertificate(domain string, cert acme.CertificateResource) error {
	// Combine certificate + private key (for haproxy)
	combined := append(cert.Certificate, cert.PrivateKey...)

	// Store combined certificate in ETCD
	if err := s.Repository.StoreDomainCertificate(domain, combined); err != nil {
		return maskAny(err)
	}

	return nil
}

// claimRequestCertificatesMutex tries to claim the distributed mutex for
// requesting certificates.
// On success it returns true with a mutex.
// When the mutex is already claimed, it returns false, nil.
// When another error occurs, this error is returned.
func (s *certificateRequester) claimRequestCertificatesMutex() (bool, *mutex.GlobalMutex, error) {
	// Create mutex
	m, err := s.mutexService.New(requestCertificatesLockName, requestCertificatesLockTTL)
	if err != nil {
		return false, nil, maskAny(err)
	}

	// Try to claim mute
	if err := m.Lock(); err != nil {
		if mutex.IsAlreadyLocked(err) {
			// Another instance has the mutex
			return false, nil, nil
		}
		return false, nil, maskAny(err)
	}

	// We've got the mutex and it is locked
	return true, m, nil
}
