package locks

type LockService interface {
	// NewLock creates a new lock with a given name.
	// The lock is initialized but not yet claimed.
	// name is the name of the new lock. This name is accessible globally in the cluster
	// ownerID is an identifier of the owner of the lock. This value must be unique within the cluster
	// lockTTL is the number of seconds before the lock will automatically be released
	NewLock(name, ownerID string, lockTTL uint64) (*Lock, error)
}

// lockService is used internal in this package to communicate
// between Lock and LockService implementation.
type lockService interface {
	// Claim tries to claim a lock with given name and assign it to the given owner.
	// If successful, it returns nil, otherwise it returns an error.
	Claim(name, ownerID string, lockTTL uint64) error

	// Update tries to update a lock with given name to the given ownerID.
	// This must be called often enough to avoid TTL expiration.
	Update(name, ownerID string, lockTTL uint64) error

	// Release releases the lock with given name from the given ownerID.
	Release(name, ownerID string) error
}
