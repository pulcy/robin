package locks

import (
	"fmt"
	"sync"
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
		EtcdClient: etcdClient,
		prefix:     prefix,
		cache:      make(map[string]localLock),
	}
}

type etcdLockService struct {
	EtcdClient *etcd.Client
	prefix     string

	cache      map[string]localLock
	cacheMutex sync.Mutex
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
	if ls.isLockedInCache(name) {
		// It is already locked by someone inside this process
		return errgo.WithCausef(nil, AlreadyLockedError, name)
	}

	_, err := ls.EtcdClient.Create(ls.key(name), ownerID, lockTTL)
	if err != nil {
		if isEtcdWithCode(err, etcdErrCodeKeyAlreadyExists) {
			return errgo.WithCausef(nil, AlreadyLockedError, name)
		}
		return maskAny(err)
	}

	// Update local cache
	ls.setLocalCache(name, ownerID, lockTTL)

	return nil
}

// Update tries to update a lock with given name to the given ownerID.
// This must be called often enough to avoid TTL expiration.
func (ls *etcdLockService) Update(name, ownerID string, lockTTL uint64) error {
	_, err := ls.EtcdClient.CompareAndSwap(ls.key(name), ownerID, lockTTL, ownerID, 0)
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

	// Update local cache
	ls.setLocalCache(name, ownerID, lockTTL)

	return nil
}

// Release releases the lock with given name from the given ownerID.
func (ls *etcdLockService) Release(name, ownerID string) error {
	// Update local cache
	ls.removeFromLocalCache(name, ownerID)

	_, err := ls.EtcdClient.CompareAndDelete(ls.key(name), ownerID, 0)
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

func (ls *etcdLockService) isLockedInCache(name string) bool {
	ls.cacheMutex.Lock()
	defer ls.cacheMutex.Unlock()

	lock, found := ls.cache[name]
	if !found {
		return false
	}
	now := time.Now()
	if now.After(lock.Expires) {
		// Lock is expired
		delete(ls.cache, name)
		return false
	}

	return true
}

func (ls *etcdLockService) setLocalCache(name, ownerID string, lockTTL uint64) {
	ls.cacheMutex.Lock()
	defer ls.cacheMutex.Unlock()

	ls.cache[name] = localLock{
		OwnerID: ownerID,
		Expires: time.Now().Add(time.Duration(lockTTL) * time.Second),
	}
}

func (ls *etcdLockService) removeFromLocalCache(name, ownerID string) {
	ls.cacheMutex.Lock()
	defer ls.cacheMutex.Unlock()

	if lock, ok := ls.cache[name]; ok {
		// Already locked, good
		if lock.OwnerID == ownerID {
			// Remove lock
			delete(ls.cache, name)
		}
	}
}
