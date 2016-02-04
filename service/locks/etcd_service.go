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

	"github.com/juju/errgo"

	"github.com/coreos/go-etcd/etcd"
)

const (
	locksPrefix = "locks"

	// https://github.com/coreos/go-etcd/issues/130
	// https://github.com/coreos/etcd/blob/master/error/error.go
	etcdErrCodeKeyAlreadyExists = 105
	etcdErrCodeKeyTestFailed    = 101
	etcdErrCodeKeyNotFound      = 100
)

// isEtcdWithCode returns true if the given error is
// and EtcdError with given error code.
func isEtcdWithCode(err error, errCode int) bool {
	if e, ok := err.(*etcd.EtcdError); ok {
		return e.ErrorCode == errCode
	}
	return false
}

// NewEtcdLockService returns a lock service implementation
// based on etcd.
func NewEtcdLockService(etcdClient *etcd.Client, prefix string) LockService {
	return &etcdLockService{
		etcdClient: etcdClient,
		prefix:     prefix,
	}
}

type etcdLockService struct {
	etcdClient *etcd.Client
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
func (ls *etcdLockService) NewLock(name, ownerID string, lockTTL uint64) (*Lock, error) {
	return newLock(name, ownerID, lockTTL, ls)
}

// Claim tries to claim a lock with given name and assign it to the given owner.
// If successful, it returns nil, otherwise it returns an error.
func (ls *etcdLockService) Claim(name, ownerID string, lockTTL uint64) error {
	_, err := ls.etcdClient.Create(ls.key(name), ownerID, lockTTL)
	if err != nil {
		if isEtcdWithCode(err, etcdErrCodeKeyAlreadyExists) {
			return errgo.WithCausef(nil, AlreadyLockedError, name)
		}
		return maskAny(err)
	}

	return nil
}

// Update tries to update a lock with given name to the given ownerID.
// This must be called often enough to avoid TTL expiration.
func (ls *etcdLockService) Update(name, ownerID string, lockTTL uint64) error {
	_, err := ls.etcdClient.CompareAndSwap(ls.key(name), ownerID, lockTTL, ownerID, 0)
	if err != nil {
		if isEtcdWithCode(err, etcdErrCodeKeyTestFailed) {
			// Lock did not have ownerID as previous value
			return errgo.WithCausef(nil, NotOwnerError, name)
		}
		if isEtcdWithCode(err, etcdErrCodeKeyNotFound) {
			// Lock did not exists
			return errgo.WithCausef(nil, NotLockedError, name)
		}
		return maskAny(err)
	}

	return nil
}

// Release releases the lock with given name from the given ownerID.
func (ls *etcdLockService) Release(name, ownerID string) error {
	_, err := ls.etcdClient.CompareAndDelete(ls.key(name), ownerID, 0)
	if err != nil {
		if isEtcdWithCode(err, etcdErrCodeKeyTestFailed) {
			// Lock did not have ownerID as previous value
			return errgo.WithCausef(nil, NotOwnerError, name)
		}
		if isEtcdWithCode(err, etcdErrCodeKeyNotFound) {
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
