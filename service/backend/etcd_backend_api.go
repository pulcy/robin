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

package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"

	"github.com/coreos/etcd/client"
	"github.com/juju/errgo"
	api "github.com/pulcy/robin-api"
)

var (
	idRegexp = regexp.MustCompile("^[a-zA-Z0-9_-]+$")
)

// Add adds a given frontend record with given ID to the list of frontends.
// If the given ID already exists, a DuplicateIDError is returned.
func (eb *etcdBackend) Add(id string, record api.FrontendRecord) error {
	if err := validateID(id); err != nil {
		return maskAny(err)
	}
	if err := record.Validate(); err != nil {
		return maskAny(err)
	}
	etcdPath := path.Join(eb.prefix, frontEndPrefix, id)
	kAPI := client.NewKeysAPI(eb.client)
	options := &client.SetOptions{
		PrevExist: client.PrevNoExist,
	}
	rawJSON, err := json.Marshal(record)
	if err != nil {
		return maskAny(err)
	}
	if _, err := kAPI.Set(context.Background(), etcdPath, string(rawJSON), options); isEtcdError(err, client.ErrorCodeNodeExist) {
		return maskAny(errgo.WithCausef(nil, api.DuplicateIDError, "Duplicate ID '%s'", id))
	} else if err != nil {
		eb.Logger.Warningf("ETCD error in Add: %#v", err)
		return maskAny(err)
	}

	return nil
}

// Remove a frontend with given ID.
// If the ID is not found, an IDNotFoundError is returned.
func (eb *etcdBackend) Remove(id string) error {
	if err := validateID(id); err != nil {
		return maskAny(err)
	}
	etcdPath := path.Join(eb.prefix, frontEndPrefix, id)
	kAPI := client.NewKeysAPI(eb.client)
	options := &client.DeleteOptions{
		Recursive: false,
	}
	_, err := kAPI.Delete(context.Background(), etcdPath, options)
	if isEtcdError(err, client.ErrorCodeKeyNotFound) {
		return maskAny(errgo.WithCausef(nil, api.IDNotFoundError, "ID '%s' not found", id))
	}
	if err != nil {
		eb.Logger.Warningf("ETCD error in Remove: %#v", err)
		return maskAny(err)
	}
	return nil
}

// All returns a map of all known frontend records mapped by their ID.
func (eb *etcdBackend) All() (map[string]api.FrontendRecord, error) {
	etcdPath := path.Join(eb.prefix, frontEndPrefix)
	kAPI := client.NewKeysAPI(eb.client)
	options := &client.GetOptions{
		Recursive: false,
		Sort:      false,
	}
	result := make(map[string]api.FrontendRecord)
	resp, err := kAPI.Get(context.Background(), etcdPath, options)
	if isEtcdError(err, client.ErrorCodeKeyNotFound) {
		return result, nil
	}
	if err != nil {
		eb.Logger.Warningf("ETCD error in All: %#v", err)
		return nil, maskAny(err)
	}
	if resp.Node == nil {
		return result, nil
	}
	for _, frontEndNode := range resp.Node.Nodes {
		id := path.Base(frontEndNode.Key)
		rawJSON := frontEndNode.Value
		record := api.FrontendRecord{}
		if err := json.Unmarshal([]byte(rawJSON), &record); err != nil {
			eb.Logger.Errorf("Cannot unmarshal registration of %s", frontEndNode.Key)
			continue
		}
		result[id] = record
	}

	return result, nil
}

// Get returns the frontend record for the given id.
// If the ID is not found, an IDNotFoundError is returned.
func (eb *etcdBackend) Get(id string) (api.FrontendRecord, error) {
	if err := validateID(id); err != nil {
		return api.FrontendRecord{}, maskAny(err)
	}
	etcdPath := path.Join(eb.prefix, frontEndPrefix, id)
	kAPI := client.NewKeysAPI(eb.client)
	options := &client.GetOptions{
		Recursive: false,
		Sort:      false,
	}
	resp, err := kAPI.Get(context.Background(), etcdPath, options)
	if isEtcdError(err, client.ErrorCodeKeyNotFound) {
		return api.FrontendRecord{}, maskAny(errgo.WithCausef(nil, api.IDNotFoundError, "ID '%s' not found", id))
	}
	if err != nil {
		eb.Logger.Warningf("ETCD error in Get: %#v", err)
		return api.FrontendRecord{}, maskAny(err)
	}
	if resp.Node == nil {
		return api.FrontendRecord{}, maskAny(errgo.WithCausef(nil, api.IDNotFoundError, "ID '%s' not found", id))
	}
	rawJSON := resp.Node.Value
	record := api.FrontendRecord{}
	if err := json.Unmarshal([]byte(rawJSON), &record); err != nil {
		return api.FrontendRecord{}, maskAny(fmt.Errorf("Cannot unmarshal registration of %s", id))
	}

	return record, nil
}

func validateID(id string) error {
	if !idRegexp.MatchString(id) {
		return maskAny(errgo.WithCausef(nil, api.ValidationError, "invalid ID '%s'", id))
	}
	return nil
}
