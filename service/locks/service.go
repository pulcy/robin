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
