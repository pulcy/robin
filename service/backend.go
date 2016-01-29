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
}

type ServiceRegistration struct {
	ServiceName   string           // Name of the service
	ServicePort   int              // Port the service is listening on (inside its container)
	Instances     ServiceInstances // List instances of the service (can not be empty)
	Selectors     ServiceSelectors // List of selectors to match traffic to this service
	HttpCheckPath string           // Path (on the service) used for health checks (can be empty)
}

func (sr ServiceRegistration) FullString() string {
	return fmt.Sprintf("%s-%d-%s-%s-%s",
		sr.ServiceName,
		sr.ServicePort,
		sr.Instances.FullString(),
		sr.Selectors.FullString(),
		sr.HttpCheckPath)
}

type ServiceRegistrations []ServiceRegistration

func (list ServiceRegistrations) Sort() {
	sort.Sort(list)
	for _, sr := range list {
		sr.Instances.Sort()
		sr.Selectors.Sort()
	}
}

type ServiceInstance struct {
	IP   string // IP address to connect to to reach the service instance
	Port int    // Port to connect to to reach the service instance
}

func (si ServiceInstance) FullString() string {
	return fmt.Sprintf("%s-%d", si.IP, si.Port)
}

type ServiceInstances []ServiceInstance

func (list ServiceInstances) FullString() string {
	slist := []string{}
	for _, si := range list {
		slist = append(slist, si.FullString())
	}
	sort.Strings(slist)
	return "[" + strings.Join(slist, ",") + "]"
}

func (list ServiceInstances) Sort() {
	sort.Sort(list)
}

type ServiceSelector struct {
	Weight     int    // How important is this selector. (0-100), 100 being most important
	Domain     string // Domain to match on
	SslCert    string // SSL certificate filename
	PathPrefix string // Prefix of HTTP path to match on
	Private    bool   // If set, match on private port (81), otherwise match of public port (80,443)
	Users      Users  // If set, require authentication for one of these users
}

func (fs ServiceSelector) FullString() string {
	users := []string{}
	for _, user := range fs.Users {
		users = append(users, user.FullString())
	}
	sort.Strings(users)
	return fmt.Sprintf("%03d-%s-%s-%s-%v-%#v", (100 - fs.Weight), fs.Domain, fs.SslCert, fs.PathPrefix, fs.Private, users)
}

type ServiceSelectors []ServiceSelector

func (list ServiceSelectors) FullString() string {
	slist := []string{}
	for _, ss := range list {
		slist = append(slist, ss.FullString())
	}
	sort.Strings(slist)
	return "[" + strings.Join(slist, ",") + "]"
}

func (list ServiceSelectors) Sort() {
	sort.Sort(list)
	for _, fs := range list {
		sort.Sort(fs.Users)
	}
}

func (list ServiceSelectors) Contains(sel ServiceSelector) bool {
	q := sel.FullString()
	for _, ss := range list {
		if ss.FullString() == q {
			return true
		}
	}
	return false
}

type User struct {
	Name         string
	PasswordHash string
}

func (user User) FullString() string {
	return fmt.Sprintf("%s-%s", user.Name, user.PasswordHash)
}

type Users []User

func (sr *ServiceRegistration) HasPublicSelectors() bool {
	for _, sel := range sr.Selectors {
		if !sel.Private {
			return true
		}
	}
	return false
}

func (sr *ServiceRegistration) HasPrivateSelectors() bool {
	for _, sel := range sr.Selectors {
		if sel.Private {
			return true
		}
	}
	return false
}

// backendName creates a valid name for the backend of this registration
// in haproxy.
func (sr *ServiceRegistration) backendName(private bool) string {
	return fmt.Sprintf("backend_%s_%d_%s", cleanName(sr.ServiceName), sr.ServicePort, visibilityPostfix(private))
}

// userListName creates a valid name for the userlist of this registration
// in haproxy.
func (sr *ServiceRegistration) userListName(selectorIndex int) string {
	return fmt.Sprintf("userlist_%s_%d_%d", cleanName(sr.ServiceName), sr.ServicePort, selectorIndex)
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
func (list ServiceSelectors) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list ServiceSelectors) Less(i, j int) bool {
	a := list[i].FullString()
	b := list[j].FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list ServiceSelectors) Swap(i, j int) {
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
func (list ServiceInstances) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list ServiceInstances) Less(i, j int) bool {
	a := list[i].FullString()
	b := list[j].FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list ServiceInstances) Swap(i, j int) {
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

type selectorServicePair struct {
	Selector ServiceSelector
	Service  ServiceRegistration
}

type selectorServicePairs []selectorServicePair

// Len is the number of elements in the collection.
func (list selectorServicePairs) Len() int {
	return len(list)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (list selectorServicePairs) Less(i, j int) bool {
	a := list[i].Selector.FullString() + list[i].Service.FullString()
	b := list[j].Selector.FullString() + list[j].Service.FullString()
	return strings.Compare(a, b) < 0
}

// Swap swaps the elements with indexes i and j.
func (list selectorServicePairs) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
