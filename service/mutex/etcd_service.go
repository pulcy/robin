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

package mutex

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/dchest/uniuri"
	"github.com/juju/errgo"
	"golang.org/x/net/context"
)

const (
	locksPrefix = "locks"
)

// isEtcdWithCode returns true if the given error is
// and ETCD Error with given error code.
func isEtcdWithCode(err error, errCode int) bool {
	if e, ok := err.(*client.Error); ok {
		return e.Code == errCode
	}
	return false
}

// NewEtcdGlobalMutexService returns a global mutex service implementation
// based on ETCD.
func NewEtcdGlobalMutexService(etcdClient client.Client, prefix string) GlobalMutexService {
	return &etcdGlobalMutexService{
		etcdClient: etcdClient,
		prefix:     prefix,
		ownerID:    uniuri.New(),
	}
}

type etcdGlobalMutexService struct {
	etcdClient client.Client
	prefix     string
	ownerID    string
}

// New creates a new global mutex with a given name.
// The mutex is initialized but not yet claimed.
// name is the name of the new mutex. This name is accessible globally in the cluster
// ttl is the amount of time before the lock will automatically be released
func (gms *etcdGlobalMutexService) New(name string, ttl time.Duration) (*GlobalMutex, error) {
	m, err := newMutex(name, ttl, gms)
	if err != nil {
		return nil, maskAny(err)
	}
	return m, nil
}

// Claim tries to claim a lock with given name and assign it to the given owner.
// If successful, it returns nil, otherwise it returns an error.
func (gms *etcdGlobalMutexService) Claim(name string, ttl time.Duration) error {
	kAPI := client.NewKeysAPI(gms.etcdClient)
	options := &client.SetOptions{
		PrevExist: client.PrevNoExist,
		TTL:       ttl,
	}
	_, err := kAPI.Set(context.Background(), gms.key(name), gms.ownerID, options)
	if err != nil {
		if isEtcdWithCode(err, client.ErrorCodeNodeExist) {
			return maskAny(errgo.WithCausef(nil, AlreadyLockedError, name))
		}
		return maskAny(err)
	}

	return nil
}

// Update tries to update a lock with given name to the given ownerID.
// This must be called often enough to avoid TTL expiration.
func (gms *etcdGlobalMutexService) Update(name string, ttl time.Duration) error {
	kAPI := client.NewKeysAPI(gms.etcdClient)
	options := &client.SetOptions{
		PrevValue: gms.ownerID,
		PrevExist: client.PrevExist,
		TTL:       ttl,
	}
	_, err := kAPI.Set(context.Background(), gms.key(name), gms.ownerID, options)
	if err != nil {
		if isEtcdWithCode(err, client.ErrorCodeTestFailed) {
			// Lock did not have ownerID as previous value
			return maskAny(errgo.WithCausef(nil, NotOwnerError, name))
		}
		if isEtcdWithCode(err, client.ErrorCodeKeyNotFound) {
			// Lock did not exists
			return maskAny(errgo.WithCausef(nil, NotLockedError, name))
		}
		return maskAny(err)
	}

	return nil
}

// Release releases the lock with given name from the given ownerID.
func (gms *etcdGlobalMutexService) Release(name string) error {
	kAPI := client.NewKeysAPI(gms.etcdClient)
	options := &client.DeleteOptions{
		PrevValue: gms.ownerID,
	}
	_, err := kAPI.Delete(context.Background(), gms.key(name), options)
	if err != nil {
		if isEtcdWithCode(err, client.ErrorCodeTestFailed) {
			// Lock did not have ownerID as previous value
			return errgo.WithCausef(nil, NotOwnerError, name)
		}
		if isEtcdWithCode(err, client.ErrorCodeKeyNotFound) {
			// Lock did not exists
			return errgo.WithCausef(nil, NotLockedError, name)
		}
		return maskAny(err)
	}
	return nil
}

func (gms *etcdGlobalMutexService) key(name string) string {
	return fmt.Sprintf("%s/%s/%s", gms.prefix, locksPrefix, name)
}
