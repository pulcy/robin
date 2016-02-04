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

package locks

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/client"
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

// NewEtcdLockService returns a lock service implementation
// based on ETCD.
func NewEtcdLockService(etcdClient client.Client, prefix string) LockService {
	return &etcdLockService{
		etcdClient: etcdClient,
		prefix:     prefix,
	}
}

type etcdLockService struct {
	etcdClient client.Client
	prefix     string
}

type localLock struct {
	OwnerID string
	Expires time.Time
}

// NewLock creates a new lock with a given name.
// The lock is initialized but not yet claimed.
// name is the name of the new lock. This name is accessible globally in the cluster
// ownerID is an identifier of the owner of the lock. This value must be unique within the cluster
// lockTTL is the numer of seconds before the lock will automatically be released
func (ls *etcdLockService) NewLock(name, ownerID string, lockTTL time.Duration) (*Lock, error) {
	l, err := newLock(name, ownerID, lockTTL, ls)
	if err != nil {
		return nil, maskAny(err)
	}
	return l, nil
}

// Claim tries to claim a lock with given name and assign it to the given owner.
// If successful, it returns nil, otherwise it returns an error.
func (ls *etcdLockService) Claim(name, ownerID string, lockTTL time.Duration) error {
	kAPI := client.NewKeysAPI(ls.etcdClient)
	options := &client.SetOptions{
		PrevExist: client.PrevNoExist,
		TTL:       lockTTL,
	}
	_, err := kAPI.Set(context.Background(), ls.key(name), ownerID, options)
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
func (ls *etcdLockService) Update(name, ownerID string, lockTTL time.Duration) error {
	kAPI := client.NewKeysAPI(ls.etcdClient)
	options := &client.SetOptions{
		PrevValue: ownerID,
		PrevExist: client.PrevExist,
		TTL:       lockTTL,
	}
	_, err := kAPI.Set(context.Background(), ls.key(name), ownerID, options)
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
func (ls *etcdLockService) Release(name, ownerID string) error {
	kAPI := client.NewKeysAPI(ls.etcdClient)
	options := &client.DeleteOptions{
		PrevValue: ownerID,
	}
	_, err := kAPI.Delete(context.Background(), ls.key(name), options)
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

func (ls *etcdLockService) key(name string) string {
	return fmt.Sprintf("%s/%s/%s", ls.prefix, locksPrefix, name)
}
