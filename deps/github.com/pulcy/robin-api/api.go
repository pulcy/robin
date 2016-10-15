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

package api

// API is the client interface for controlling the frontends of the Robin loadbalancer.
type API interface {
	// Add adds a given frontend record with given ID to the list of frontends.
	// If the given ID already exists, a DuplicateIDError is returned.
	Add(id string, record FrontendRecord) error

	// Remove a frontend with given ID.
	// If the ID is not found, an IDNotFoundError is returned.
	Remove(id string) error

	// All returns a map of all known frontend records mapped by their ID.
	All() (map[string]FrontendRecord, error)

	// Get returns the frontend record for the given id.
	// If the ID is not found, an IDNotFoundError is returned.
	Get(id string) (FrontendRecord, error)
}
