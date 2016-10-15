package api

import (
	"fmt"
	"net/url"

	"github.com/pulcy/rest-kit"
)

type client struct {
	rc *restkit.RestClient
}

// NewClient creates a new API implementation for the given base URL.
func NewClient(baseURL *url.URL) (API, error) {
	return &client{
		rc: restkit.NewRestClient(baseURL),
	}, nil
}

// Add adds a given frontend record with given ID to the list of frontends.
// If the given ID already exists, a DuplicateIDError is returned.
func (c *client) Add(id string, record FrontendRecord) error {
	if err := c.rc.Request("POST", fmt.Sprintf("/v1/frontend/%s", id), nil, record, nil); err != nil {
		return maskAny(err)
	}
	return nil
}

// Remove a frontend with given ID.
// If the ID is not found, an IDNotFoundError is returned.
func (c *client) Remove(id string) error {
	if err := c.rc.Request("DELETE", fmt.Sprintf("/v1/frontend/%s", id), nil, nil, nil); err != nil {
		return maskAny(err)
	}
	return nil
}

// All returns a map of all known frontend records mapped by their ID.
func (c *client) All() (map[string]FrontendRecord, error) {
	var result map[string]FrontendRecord
	if err := c.rc.Request("GET", "/v1/frontend", nil, nil, &result); err != nil {
		return nil, maskAny(err)
	}
	return result, nil
}

// Get returns the frontend record for the given id.
// If the ID is not found, an IDNotFoundError is returned.
func (c *client) Get(id string) (FrontendRecord, error) {
	var result FrontendRecord
	if err := c.rc.Request("GET", fmt.Sprintf("/v1/frontend/%s", id), nil, nil, &result); err != nil {
		return FrontendRecord{}, maskAny(err)
	}
	return result, nil
}
