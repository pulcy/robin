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
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/op/go-logging"
)

type CertificatesFileCache interface {
	Clear()

	// GetDomainCertificatePath returns the path of a certificate file for the given domain.
	GetDomainCertificatePath(domain string) (string, error)
}

type certificatesFileCache struct {
	TmpCertificatePath string // Path of directory where temporary certificates are written to.
	Repository         CertificatesRepository
	Logger             *logging.Logger

	domainFileCache      map[string]string
	domainFileCacheMutex sync.Mutex
}

func NewCertificatesFileCache(tmpPath string, repository CertificatesRepository, logger *logging.Logger) CertificatesFileCache {
	return &certificatesFileCache{
		TmpCertificatePath: tmpPath,
		Repository:         repository,
		Logger:             logger,
		domainFileCache:    make(map[string]string),
	}
}

func (s *certificatesFileCache) Clear() {
	s.domainFileCacheMutex.Lock()
	defer s.domainFileCacheMutex.Unlock()

	s.domainFileCache = make(map[string]string)
	s.Logger.Debugf("Cleared domain file cache")
}

// getDomainCertificatePath returns the path of a certificate file for the given domain.
func (s *certificatesFileCache) GetDomainCertificatePath(domain string) (string, error) {
	s.domainFileCacheMutex.Lock()
	defer s.domainFileCacheMutex.Unlock()

	if path, ok := s.domainFileCache[domain]; ok {
		// File path found in cache
		return path, nil
	}

	// Not found in cache, try repository
	certificate, err := s.Repository.LoadDomainCertificate(domain)
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
