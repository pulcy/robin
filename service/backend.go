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
	ServiceName string   // Name of the service
	Port        int      // Port the service is listening on (host port)
	Backends    []string // List of ip:port for the backend of this service
}

type FrontEndRegistration struct {
	Name          string
	Selectors     []FrontEndSelector
	Service       string
	Port          int
	HttpCheckPath string
}

type FrontEndSelector struct {
	Domain     string
	SslCert    string
	PathPrefix string
	Port       int
	Private    bool
	Users      []User
}

type User struct {
	Name         string
	PasswordHash string
}

// Does the given frontend registration match the given service registration?
func (fr FrontEndRegistration) Match(sr ServiceRegistration) bool {
	if fr.Service != sr.ServiceName {
		return false
	}
	if fr.Port == 0 {
		// No port specified, use all ports
		return true
	}
	return fr.Port == sr.Port
}

// backendName creates a valid name for the backend of this registration
// in haproxy.
func (fr *FrontEndRegistration) backendName() string {
	return fmt.Sprintf("backend_%s_%d", cleanName(fr.Name), fr.Port)
}

// aclName creates a valid name for the acl of this registration
// in haproxy.
func (fr *FrontEndRegistration) aclName() string {
	return fmt.Sprintf("acl_%s_%d", cleanName(fr.Name), fr.Port)
}

// userListName creates a valid name for the userlist of this registration
// in haproxy.
func (fr *FrontEndRegistration) userListName(selectorIndex int) string {
	return fmt.Sprintf("userlist_%s_%d_%d", cleanName(fr.Name), fr.Port, selectorIndex)
}

// cleanName removes invalid characters (for haproxy conf) from the given name
func cleanName(s string) string {
	return s // TODO
}
