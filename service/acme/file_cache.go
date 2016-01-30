package acme

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func (s *acmeService) clearDomainFileCache() {
	s.domainFileCacheMutex.Lock()
	defer s.domainFileCacheMutex.Unlock()

	s.domainFileCache = make(map[string]string)
	s.Logger.Debug("Cleared domain file cache")
}

// getDomainCertificatePath returns the path of a certificate file for the given domain.
func (s *acmeService) getDomainCertificatePath(domain string) (string, error) {
	s.domainFileCacheMutex.Lock()
	defer s.domainFileCacheMutex.Unlock()

	if path, ok := s.domainFileCache[domain]; ok {
		// File path found in cache
		return path, nil
	}

	// Not found in cache, try repository
	certificate, err := s.loadDomainCertificate(domain)
	if err != nil {
		return "", maskAny(err)
	}
	if certificate == nil {
		// No certificate found
		return "", nil
	}

	// Create file path
	os.MkdirAll(s.TmpCertificatePath, 0755)
	path := filepath.Join(s.TmpCertificatePath, domain+".pem")

	// Save certificate to disk
	if err := ioutil.WriteFile(path, certificate, 0600); err != nil {
		return "", maskAny(err)
	}

	// Put in cache
	s.domainFileCache[domain] = path

	return path, nil
}
