package acme

import (
	"fmt"
	"time"

	"github.com/dchest/uniuri"
	"github.com/xenolf/lego/acme"

	"git.pulcy.com/pulcy/load-balancer/service/locks"
)

const (
	requestCertificatesLockName = "requestCertificates"
	requestCertificatesLockTTL  = 30 // sec
	requestDelay                = time.Second * 5
)

var (
	lockOwnerID = uniuri.New()
)

func (s *acmeService) requestCertificates(domains []string) error {
	isMaster, lock, err := s.claimRequestCertificatesLock()
	if err != nil {
		return maskAny(err)
	}
	if !isMaster {
		s.Logger.Debug("requestCertificates ends because another instance is requesting certificates")
		return nil
	}

	// We're the master, let's request some certificates
	defer lock.Release()

	// Wait a bit to give haproxy the time to restart
	time.Sleep(requestDelay)

	failedDomains := []string{}
	for _, domain := range domains {
		bundle := true
		certificates, failures := s.acmeClient.ObtainCertificate([]string{domain}, bundle, s.privateKey)
		if len(failures) > 0 {
			failedDomains = append(failedDomains, domain)
			s.Logger.Error("ObtainCertificate for '%s' failed: %#v", domain, failures)
			continue
		}

		// Store the domain so all instances can use it
		if err := s.saveCertificate(domain, certificates); err != nil {
			s.Logger.Error("Failed to save certificate for '%s': %#v", domain, err)
		} else {

		}
	}

	if len(failedDomains) > 0 {
		return maskAny(fmt.Errorf("Failed to obtain certificates for %#v", failedDomains))
	}
	return nil
}

// saveCertificate stores the given certificate in ETCD.
func (s *acmeService) saveCertificate(domain string, cert acme.CertificateResource) error {
	// Combine certificate + private key (for haproxy)
	combined := append(cert.Certificate, cert.PrivateKey...)

	// Store combined certificate in ETCD
	if err := s.storeDomainCertificate(domain, combined); err != nil {
		return maskAny(err)
	}

	return nil
}

// claimRequestCertificatesLock tries to claim the distributed lock for
// requesting certificates.
// On success it returns true with a lock.
// When the lock is already claimed, it returns false, nil.
// When another error occurs, this error is returned.
func (s *acmeService) claimRequestCertificatesLock() (bool, *locks.Lock, error) {
	// Create lock
	lock, err := s.LockService.NewLock(requestCertificatesLockName, lockOwnerID, requestCertificatesLockTTL)
	if err != nil {
		return false, nil, maskAny(err)
	}

	// Try to claim lock
	if err := lock.Claim(); err != nil {
		if locks.IsAlreadyLocked(err) {
			// Another instance has the lock
			return false, nil, nil
		}
		return false, nil, maskAny(err)
	}

	// We've got the lock
	return true, lock, nil
}
