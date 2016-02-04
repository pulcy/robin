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
	"encoding/base64"
	"path"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const (
	etcdCertificatesFolder = "certificates"
)

func NewEtcdCertificatesRepository(etcdPrefix string, etcdClient client.Client) CertificatesRepository {
	kAPI := client.NewKeysAPI(etcdClient)
	options := &client.WatcherOptions{
		Recursive: true,
	}
	prefix := path.Join(etcdPrefix, etcdCertificatesFolder)
	watcher := kAPI.Watcher(prefix, options)
	return &etcdCertificatesRepository{
		EtcdPrefix:                etcdPrefix,
		EtcdClient:                etcdClient,
		domainCertificatesWatcher: watcher,
	}
}

type etcdCertificatesRepository struct {
	EtcdPrefix string
	EtcdClient client.Client

	domainCertificatesWatcher client.Watcher
}

// isEtcdWithCode returns true if the given error is
// and EtcdError with given error code.
func isEtcdWithCode(err error, errCode int) bool {
	if e, ok := err.(*client.Error); ok {
		return e.Code == errCode
	}
	return false
}

// watchDomainCertificates waits for changes on one of the domain certificates
// in the repository and returns where there is a change.
func (s *etcdCertificatesRepository) WatchDomainCertificates() error {
	_, err := s.domainCertificatesWatcher.Next(context.Background())
	if err != nil {
		return maskAny(err)
	}
	return nil
}

// loadDomainCertificate tries to load the certificate for the given domain from the ETCD repository
// Returns nil,nil if domain is not found.
func (s *etcdCertificatesRepository) LoadDomainCertificate(domain string) ([]byte, error) {
	kAPI := client.NewKeysAPI(s.EtcdClient)
	options := &client.GetOptions{
		Recursive: false,
		Sort:      false,
	}
	key := s.domainCertificateKey(domain)
	resp, err := kAPI.Get(context.Background(), key, options)
	if err != nil {
		if isEtcdWithCode(err, client.ErrorCodeKeyNotFound) {
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
	kAPI := client.NewKeysAPI(s.EtcdClient)
	options := &client.SetOptions{
		TTL: 0,
	}
	key := s.domainCertificateKey(domain)
	value := base64.StdEncoding.EncodeToString(certificate)
	if _, err := kAPI.Set(context.Background(), key, value, options); err != nil {
		return maskAny(err)
	}
	return nil
}

// domainKey creates an ETCD key for the certificate of the given domain
func (s *etcdCertificatesRepository) domainCertificateKey(domain string) string {
	return path.Join(s.EtcdPrefix, etcdCertificatesFolder, domain)
}
