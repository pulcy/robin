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
	"sync"
	"time"

	"github.com/juju/errgo"
)

type Lock struct {
	name     string        // Name of the object to lock
	ownerID  string        // Identifier of the owner of the lock
	lockTTL  time.Duration // Timespan before the lock will expire automatically
	used     bool          // Set to true once it has been claim, cannot be reclaimed afterwards
	locked   bool          // Is this lock currently locked?
	mutex    sync.Mutex    // Used to protect local access to this locks values
	service  lockService   // Internal service link
	released chan struct{} // Channel to signal release action on
}

// newLock creates and initializes a new Lock.
func newLock(name, ownerID string, lockTTL time.Duration, service lockService) (*Lock, error) {
	if name == "" {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "name empty")
	}
	if ownerID == "" {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "ownerID empty")
	}
	if lockTTL <= 0 {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "lockTTL <= 0")
	}
	if service == nil {
		return nil, errgo.WithCausef(nil, InvalidArgumentError, "service nil")
	}
	return &Lock{
		name:    name,
		ownerID: ownerID,
		lockTTL: lockTTL,
		service: service,
	}, nil
}

// Claim tries to claim the given lock. If successful, it returns nil,
// otherwise it returns an error.
// If the lock is already claimed, it returns directly with nil.
func (l *Lock) Claim() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.locked {
		// Already locked
		return nil
	}

	if l.used {
		// We cannot re-use locks
		return maskAny(AlreadyUsedError)
	}

	// Call service to lock me
	if err := l.service.Claim(l.name, l.ownerID, l.lockTTL); err != nil {
		// Claim failed
		return maskAny(err)
	}

	// Claim succeeded
	l.locked = true
	// We've now been claimed once, prevent future claims
	l.used = true

	// Prepare update loop
	l.released = make(chan struct{})
	go l.updateLoop(l.released)

	return nil
}

// Release releases the given lock.
// If the lock was not locked, it returns nil right away.
func (l *Lock) Release() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if !l.locked {
		// Not locked
		return nil
	}

	// Mark this lock as being release.
	// Doing this here (before removing the actual lock) ensures that
	// the updateLoop does not try to update the lock again.
	// If `l.service.Release` fails (later in this function) the
	// lock will expire on its own due to its TTL.
	l.locked = false

	// Close update loop
	l.released <- struct{}{}
	close(l.released)
	l.released = nil

	// Call service to unlock me
	if err := l.service.Release(l.name, l.ownerID); err != nil {
		// Release failed
		return maskAny(err)
	}

	return nil
}

// Locked returns true if this lock is claimed successfully.
func (l *Lock) Locked() bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	return l.locked
}

// updateLoop keeps updating the lock until it is released
// note that the released channel is passed as variable
// so we're sure we run on the right channel.
func (l *Lock) updateLoop(released chan struct{}) {
	for {
		select {
		case <-time.After(time.Duration((l.lockTTL/2)-1) * time.Second):
			if err := l.update(); err != nil {
				// This is really bad, we cannot update the lock
				// so it may expire on its own.
				// Since there are likely parallel processes that
				// expect that we still hold the lock, let's panic
				// here to stop those processes.
				panic(fmt.Sprintf("Cannot update lock '%s' owned by '%s: %#v", l.name, l.ownerID, err))
			}
			break
		case <-released:
			// Lock has been released, we're done
			return
		}
	}
}

// update tries to update the existing lock
func (l *Lock) update() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Only update when we're still locked
	if !l.locked {
		return nil
	}

	if err := l.service.Update(l.name, l.ownerID, l.lockTTL); err != nil {
		return maskAny(err)
	}
	return nil
}
