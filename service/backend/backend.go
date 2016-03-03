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
	Mode          string           // http|tcp
	Sticky        bool             // Switched blancing mode to source
}

func (sr ServiceRegistration) FullString() string {
	return fmt.Sprintf("%s-%d-%s-%s-%s-%s-%v",
		sr.ServiceName,
		sr.ServicePort,
		sr.Instances.FullString(),
		sr.Selectors.FullString(),
		sr.HttpCheckPath,
		sr.Mode,
		sr.Sticky)
}

func (sr ServiceRegistration) IsHttp() bool {
	return sr.Mode == "http" || sr.Mode == ""
}

func (sr ServiceRegistration) IsTcp() bool {
	return sr.Mode == "tcp"
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
	Weight            int    // How important is this selector. (0-100), 100 being most important
	Domain            string // Domain to match on
	SslCertName       string // SSL certificate filename
	TmpSslCertPath    string // Path of generated certificate file
	PathPrefix        string // Prefix of HTTP path to match on
	Private           bool   // If set, match on private port (81), otherwise match of public port (80,443)
	Users             Users  // If set, require authentication for one of these users
	AllowUnauthorized bool   // If set, allow all for this path
}

func (fs ServiceSelector) FullString() string {
	users := []string{}
	for _, user := range fs.Users {
		users = append(users, user.FullString())
	}
	sort.Strings(users)
	return fmt.Sprintf("%03d-%s-%s-%s-%v-%#v-%v", (100 - fs.Weight), fs.Domain, fs.SslCertName, fs.PathPrefix, fs.Private, users, fs.AllowUnauthorized)
}

func (ss ServiceSelector) IsSecure() bool {
	return ss.SslCertName != "" || ss.TmpSslCertPath != ""
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
