package acme

type CertificatesRepository interface {
	WatchDomainCertificates() error

	// loadDomainCertificate tries to load the certificate for the given domain from the ETCD repository
	// Returns nil,nil if domain is not found.
	LoadDomainCertificate(domain string) ([]byte, error)

	// storeDomainCertificate stores the certificate for the given domain in the ETCD repository
	StoreDomainCertificate(domain string, certificate []byte) error
}
