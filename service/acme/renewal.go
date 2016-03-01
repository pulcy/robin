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
	"sync"
	"time"

	"github.com/giantswarm/retry-go"
	"github.com/op/go-logging"
	"github.com/xenolf/lego/acme"
)

const (
	renewDaysBefore = 14
	renewalSleep    = time.Hour * 2
)

type RenewalMonitor interface {
	SetUsedDomains(domains []string)
	Start()
}

type renewalMonitor struct {
	Logger     *logging.Logger
	Repository CertificatesRepository
	Requester  CertificateRequester

	usedDomains      []string
	usedDomainsMutex sync.Mutex
}

func NewRenewalMonitor(logger *logging.Logger, repository CertificatesRepository, requester CertificateRequester) RenewalMonitor {
	return &renewalMonitor{
		Logger:     logger,
		Repository: repository,
		Requester:  requester,
	}
}

func (rm *renewalMonitor) SetUsedDomains(domains []string) {
	rm.usedDomainsMutex.Lock()
	defer rm.usedDomainsMutex.Unlock()
	rm.usedDomains = domains
}

func (rm *renewalMonitor) getUsedDomains() []string {
	rm.usedDomainsMutex.Lock()
	defer rm.usedDomainsMutex.Unlock()
	return append([]string{}, rm.usedDomains...)
}

// Start spawns a go routine to monitor for certificates that are close to their
// expiration date. Once found, it will request replacements for those certificates.
func (rm *renewalMonitor) Start() {
	go func() {
		for {
			// Get all used domains
			domains := rm.getUsedDomains()
			for _, domain := range domains {
				if err := rm.renewCertificateIfNeeded(domain); err != nil {
					rm.Logger.Errorf("Failed to renew certificate for '%s': %#v", domain, err)
				}
			}

			// Wait a bit before checking for renewals again
			if len(domains) == 0 {
				time.Sleep(time.Second * 10)
			} else {
				time.Sleep(renewalSleep)
			}
		}
	}()
}

func (rm *renewalMonitor) renewCertificateIfNeeded(domain string) error {
	// Load current certificate
	cert, err := rm.Repository.LoadDomainCertificate(domain)
	if err != nil {
		return maskAny(err)
	}
	if cert == nil {
		// Domain certificate not found, nothing to renewal
		rm.Logger.Debugf("no certificate found for '%s', so nothing to renew", domain)
		return nil
	}

	// Get expiration time of certificate
	expTime, err := acme.GetPEMCertExpiration(cert)
	if err != nil {
		return maskAny(err)
	}

	// The time returned from the certificate is always in UTC.
	// So calculate the time left with local time as UTC.
	// Directly convert it to days for the following checks.
	daysLeft := int(expTime.Sub(time.Now().UTC()).Hours() / 24)

	if daysLeft > renewDaysBefore {
		// No need to renew yet
		rm.Logger.Debugf("No need to renew certificate for '%s', it has %d days left", domain, daysLeft)
		return nil
	}

	// We need to renew the certificate
	rm.Logger.Debugf("Certificate for '%s' is due for renewal, it has %d days left", daysLeft)

	op := func() error {
		return maskAny(rm.Requester.RequestCertificates([]string{domain}))
	}

	if err := retry.Do(op,
		retry.RetryChecker(IsNotMaster),
		retry.MaxTries(15),
		retry.Sleep(time.Second*5),
		retry.Timeout(0)); err != nil {
		return maskAny(err)
	}

	return nil
}
