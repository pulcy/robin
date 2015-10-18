package service

import (
	"fmt"
)

type Backend interface {
	// Watch for changes in the backend and return where there is a change.
	Watch() error

	// Load all registered services
	Services() ([]ServiceRegistration, error)

	// Load all registered front-ends
	FrontEnds() ([]FrontEndRegistration, error)
}

type ServiceRegistration struct {
	Name     string
	Backends []string
}

type FrontEndRegistration struct {
	Name          string
	Selectors     []FrontEndSelector
	Service       string
	HttpCheckPath string
}

type FrontEndSelector struct {
	Domain      string
	SslCert     string
	PathPrefix  string
	PrivatePort int
}

// backendName creates a valid name for the backend of this registration
// in haproxy.
func (fr *FrontEndRegistration) backendName() string {
	return fmt.Sprintf("backend_%s", cleanName(fr.Name))
}

// aclName creates a valid name for the acl of this registration
// in haproxy.
func (fr *FrontEndRegistration) aclName() string {
	return fmt.Sprintf("acl_%s", cleanName(fr.Name))
}

// IsPrivate returns true if this front-end is intended to be bound to a private network only.
func (f *FrontEndSelector) IsPrivate() bool {
	return f.PrivatePort != 0
}

// cleanName removes invalid characters (for haproxy conf) from the given name
func cleanName(s string) string {
	return s // TODO
}
