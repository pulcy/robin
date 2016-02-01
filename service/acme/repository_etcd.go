package acme

import (
	"encoding/base64"
	"path"

	"github.com/coreos/go-etcd/etcd"
)

const (
	etcdCertificatesFolder = "certificates"
	etcdErrCodeKeyNotFound = 100
)

func NewEtcdCertificatesRepository(etcdPrefix string, etcdClient *etcd.Client) CertificatesRepository {
	return &etcdCertificatesRepository{
		EtcdPrefix: etcdPrefix,
		EtcdClient: etcdClient,
	}
}

type etcdCertificatesRepository struct {
	EtcdPrefix string
	EtcdClient *etcd.Client

	domainCertificatesWaitIndex uint64
}

// isEtcdWithCode returns true if the given error is
// and EtcdError with given error code.
func isEtcdWithCode(err error, errCode int) bool {
	if e, ok := err.(*etcd.EtcdError); ok {
		return e.ErrorCode == errCode
	}
	return false
}

// watchDomainCertificates waits for changes on one of the domain certificates
// in the repository and returns where there is a change.
func (s *etcdCertificatesRepository) WatchDomainCertificates() error {
	prefix := path.Join(s.EtcdPrefix, etcdCertificatesFolder)
	resp, err := s.EtcdClient.Watch(prefix, s.domainCertificatesWaitIndex, true, nil, nil)
	if err != nil {
		return maskAny(err)
	}
	s.domainCertificatesWaitIndex = resp.EtcdIndex + 1
	return nil
}

// loadDomainCertificate tries to load the certificate for the given domain from the ETCD repository
// Returns nil,nil if domain is not found.
func (s *etcdCertificatesRepository) LoadDomainCertificate(domain string) ([]byte, error) {
	key := s.domainCertificateKey(domain)
	resp, err := s.EtcdClient.Get(key, false, false)
	if err != nil {
		if isEtcdWithCode(err, etcdErrCodeKeyNotFound) {
			return nil, nil
		}
		return nil, maskAny(err)
	}
	raw, err := base64.StdEncoding.DecodeString(resp.Node.Value)
	if err != nil {
		return nil, maskAny(err)
	}
	return raw, nil
}

// storeDomainCertificate stores the certificate for the given domain in the ETCD repository
func (s *etcdCertificatesRepository) StoreDomainCertificate(domain string, certificate []byte) error {
	key := s.domainCertificateKey(domain)
	value := base64.StdEncoding.EncodeToString(certificate)
	if _, err := s.EtcdClient.Set(key, value, 0); err != nil {
		return maskAny(err)
	}
	return nil
}

// domainKey creates an ETCD key for the certificate of the given domain
func (s *etcdCertificatesRepository) domainCertificateKey(domain string) string {
	return path.Join(s.EtcdPrefix, etcdCertificatesFolder, domain)
}
