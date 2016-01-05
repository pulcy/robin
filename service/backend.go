package service

import (
	"fmt"
	"sort"
	"strings"
)

type Backend interface {
	// Watch for changes in the backend and return where there is a change.
	Watch() error

	// Load all registered services
	Services() (ServiceRegistrations, error)

	// Load all registered front-ends
	FrontEnds() (FrontEndRegistrations, error)
}

type ServiceRegistration struct {
	ServiceName string   // Name of the service
	Port        int      // Port the service is listening on (host port)
	Backends    []string // List of ip:port for the backend of this service
}

func (sr ServiceRegistration) FullString() string {
	return fmt.Sprintf("%s-%d-%#v", sr.ServiceName, sr.Port, sr.Backends)
}

type ServiceRegistrations []ServiceRegistration

func (list ServiceRegistrations) Sort() {
	sort.Sort(list)
	for _, sr := range list {
		sort.Strings(sr.Backends)
	}
}

type FrontEndRegistration struct {
	Name          string
	Selectors     FrontEndSelectors
	Service       string
	Port          int
	HttpCheckPath string
}

func (fr FrontEndRegistration) FullString() string {
	return fmt.Sprintf("%s-%s-%d-%s-%#v", fr.Name, fr.Service, fr.Port, fr.HttpCheckPath, fr.Selectors)
}

type FrontEndRegistrations []FrontEndRegistration

func (list FrontEndRegistrations) Sort() {
	sort.Sort(list)
	for _, fr := range list {
		fr.Selectors.Sort()
	}
}

type FrontEndSelector struct {
	Domain     string
	SslCert    string
	PathPrefix string
	Port       int
	Private    bool
	Users      Users
}

func (fs FrontEndSelector) FullString() string {
	users := []string{}
	for _, user := range fs.Users {
		users = append(users, user.FullString())
	}
	sort.Strings(users)
	return fmt.Sprintf("%s-%s-%s-%d-%v-%#v", fs.Domain, fs.SslCert, fs.PathPrefix, fs.Port, fs.Private, users)
}

type FrontEndSelectors []FrontEndSelector

func (list FrontEndSelectors) Sort() {
	sort.Sort(list)
	for _, fs := range list {
		sort.Sort(fs.Users)
	}
}

type User struct {
	Name         string
	PasswordHash string
}

func (user User) FullString() string {
	return fmt.Sprintf("%s-%s", user.Name, user.PasswordHash)
}

type Users []User

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

func (fr *FrontEndRegistration) HasPublicSelectors() bool {
	for _, sel := range fr.Selectors {
		if !sel.Private {
			return true
		}
	}
	return false
}

func (fr *FrontEndRegistration) HasPrivateSelectors() bool {
	for _, sel := range fr.Selectors {
		if sel.Private {
			return true
		}
	}
	return false
}

// backendName creates a valid name for the backend of this registration
// in haproxy.
func (fr *FrontEndRegistration) backendName(private bool) string {
	return fmt.Sprintf("backend_%s_%d_%s", cleanName(fr.Name), fr.Port, visibilityPostfix(private))
}

// aclName creates a valid name for the acl of this registration
// in haproxy.
func (fr *FrontEndRegistration) aclName(private bool) string {
	return fmt.Sprintf("acl_%s_%d_%s", cleanName(fr.Name), fr.Port, visibilityPostfix(private))
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

func visibilityPostfix(private bool) string {
	if private {
		return "private"
	}
	return "public"
}

// Len is the number of elements in the collection.
func (list FrontEndRegistrations) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list FrontEndRegistrations) Less(i, j int) bool {
	a := list[i].FullString()
	b := list[j].FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list FrontEndRegistrations) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Len is the number of elements in the collection.
func (list FrontEndSelectors) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list FrontEndSelectors) Less(i, j int) bool {
	a := list[i].FullString()
	b := list[j].FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list FrontEndSelectors) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Len is the number of elements in the collection.
func (list ServiceRegistrations) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list ServiceRegistrations) Less(i, j int) bool {
	a := list[i].FullString()
	b := list[j].FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list ServiceRegistrations) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

// Len is the number of elements in the collection.
func (list Users) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list Users) Less(i, j int) bool {
	a := list[i].FullString()
	b := list[j].FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list Users) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
