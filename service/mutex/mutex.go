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
	"sync"
	"time"

	"github.com/juju/errgo"
)

type GlobalMutex struct {
	name     string        // Name of the object to gard
	ttl      time.Duration // Amount of time before the mutex will expire automatically
	used     bool          // Set to true once it has been claim, cannot be reclaimed afterwards
	locked   bool          // Is this lock currently locked?
	mutex    sync.Mutex    // Used to protect local access to this locks values
	service  mutexService  // Internal service link
	released chan struct{} // Channel to signal release action on
}

// newMutex creates and initializes a new GlobalMutex.
func newMutex(name string, ttl time.Duration, service mutexService) (*GlobalMutex, error) {
	if name == "" {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "name empty")
	}
	if ttl <= 0 {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "ttl <= 0")
	}
	if service == nil {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "service nil")
	}
	return &GlobalMutex{
		name:    name,
		ttl:     ttl,
		service: service,
	}, nil
}

// Lock tries to claim the given mutex. If successful, it returns nil,
// otherwise it returns an error.
// If the mutex is already locked, it returns directly with nil.
func (gm *GlobalMutex) Lock() error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if gm.locked {
		// Already locked
		return nil
	}

	if gm.used {
		// We cannot re-use locks
		return maskAny(AlreadyUsedError)
	}

	// Call service to lock me
	if err := gm.service.Claim(gm.name, gm.ttl); err != nil {
		// Claim failed
		return maskAny(err)
	}

	// Claim succeeded
	gm.locked = true
	// We've now been claimed once, prevent future claims
	gm.used = true

	// Prepare update loop
	gm.released = make(chan struct{})
	go gm.updateLoop(gm.released)

	return nil
}

// Unlock releases the given mutex.
// If the mutex was not locked, it returns nil right away.
func (gm *GlobalMutex) Unlock() error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if !gm.locked {
		// Not locked
		return nil
	}

	// Mark this mutex as being released.
	// Doing this here (before removing the actual lock) ensures that
	// the updateLoop does not try to update the lock again.
	// If `l.service.Release` fails (later in this function) the
	// lock will expire on its own due to its TTL.
	gm.locked = false

	// Close update loop
	gm.released <- struct{}{}
	close(gm.released)
	gm.released = nil

	// Call service to unlock me
	if err := gm.service.Release(gm.name); err != nil {
		// Release failed
		return maskAny(err)
	}

	return nil
}

// Locked returns true if this lock is claimed successfully.
func (gm *GlobalMutex) Locked() bool {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	return gm.locked
}

// updateLoop keeps updating the lock until it is released
// note that the released channel is passed as variable
// so we're sure we run on the right channel.
func (gm *GlobalMutex) updateLoop(released chan struct{}) {
	for {
		select {
		case <-time.After(time.Duration((gm.ttl/2)-1) * time.Second):
			if err := gm.update(); err != nil {
				// This is really bad, we cannot update the mutex
				// so it may expire on its own.
				// Since there are likely parallel processes that
				// expect that we still hold the lock, let's panic
				// here to stop those processes.
				panic(fmt.Sprintf("Cannot update mutex '%s': %#v", gm.name, err))
			}
			break
		case <-released:
			// Lock has been released, we're done
			return
		}
	}
}

// update tries to update the existing lock
func (gm *GlobalMutex) update() error {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	// Only update when we're still locked
	if !gm.locked {
		return nil
	}

	if err := gm.service.Update(gm.name, gm.ttl); err != nil {
		return maskAny(err)
	}
	return nil
}
