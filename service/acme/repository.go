package acme

import (
	"encoding/base64"
	"path"
)

const (
	etcdCertificatesFolder = "certificates"
)

// Watch for changes on one of the domain certificates in the repository and returns where there is a change.
func (s *acmeService) WatchDomainCertificates() error {
	prefix := path.Join(s.EtcdPrefix, etcdCertificatesFolder)
	resp, err := s.EtcdClient.Watch(prefix, s.domainCertificatesWaitIndex, true, nil, nil)
	if err != nil {
		return maskAny(err)
	} else {
		s.domainCertificatesWaitIndex = resp.EtcdIndex + 1
		return nil
	}
}

// loadDomainCertificate tries to load the certificate for the given domain from the ETCD repository
func (s *acmeService) loadDomainCertificate(domain string) ([]byte, error) {
	key := s.domainCertificateKey(domain)
	resp, err := s.EtcdClient.Get(key, false, false)
	if err != nil {
		return nil, maskAny(err)
	}
	raw, err := base64.StdEncoding.DecodeString(resp.Node.Value)
	if err != nil {
		return nil, maskAny(err)
	}
	return raw, nil
}

// storeDomainCertificate stores the certificate for the given domain in the ETCD repository
func (s *acmeService) storeDomainCertificate(domain string, certificate []byte) error {
	key := s.domainCertificateKey(domain)
	value := base64.StdEncoding.EncodeToString(certificate)
	if _, err := s.EtcdClient.Set(key, value, 0); err != nil {
		return maskAny(err)
	}
	return nil
}

// domainKey creates an ETCD key for the certificate of the given domain
func (s *acmeService) domainCertificateKey(domain string) string {
	return path.Join(s.EtcdPrefix, etcdCertificatesFolder, domain)
}
