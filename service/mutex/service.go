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
	"time"
)

type GlobalMutexService interface {
	// New creates a new global mutex with a given name.
	// The mutex is initialized but not yet claimed.
	// name is the name of the new mutex. This name is accessible globally in the cluster
	// ttl is the amount of time before the mutex will automatically be released
	New(name string, ttl time.Duration) (*GlobalMutex, error)
}

// mutexService is used internal in this package to communicate
// between GlobalMutex and GlobalMutexService implementation.
type mutexService interface {
	// Claim tries to claim a mutex with given name.
	// If successful, it returns nil, otherwise it returns an error.
	Claim(name string, ttl time.Duration) error

	// Update tries to update a mutex with given name.
	// This must be called often enough to avoid TTL expiration.
	Update(name string, ttl time.Duration) error

	// Release releases the mutex with given name from the given ownerID.
	Release(name string) error
}
