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

package service

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"git.pulcy.com/pulcy/load-balancer/haproxy"
	"git.pulcy.com/pulcy/load-balancer/service/backend"
)

var (
	globalOptions = []string{
		//"log global",
		"tune.ssl.default-dh-param 2048",
		"ssl-default-bind-ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-DSS-AES128-GCM-SHA256:kEDH+AESGCM:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA:ECDHE-ECDSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-DSS-AES128-SHA256:DHE-RSA-AES256-SHA256:DHE-DSS-AES256-SHA:DHE-RSA-AES256-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128:AES256:AES:CAMELLIA:!aNULL:!eNULL:!EXPORT:!DES:!RC4:!MD5:!PSK:!aECDH:!EDH-DSS-DES-CBC3-SHA:!EDH-RSA-DES-CBC3-SHA:!KRB5-DES-CBC3-SHA",
	}
	defaultsOptions = []string{
		"mode http",
		"timeout connect 5000ms",
		"timeout client 50000ms",
		"timeout server 50000ms",
		"option forwardfor",
		"option http-server-close",
		"log global",
		"option httplog",
		"option dontlognull",
		"errorfile 400 /app/errors/400.http",
		"errorfile 403 /app/errors/403.http",
		"errorfile 408 /app/errors/408.http",
		"errorfile 500 /app/errors/500.http",
		"errorfile 502 /app/errors/502.http",
		"errorfile 503 /app/errors/503.http",
		"errorfile 504 /app/errors/504.http",
	}
)

type useBlock struct {
	AclNames    []string
	AuthAclName string
	Service     backend.ServiceRegistration
}

type selection struct {
	Private bool
	Http    bool
}

// renderConfig creates a new haproxy configuration content.
func (s *Service) renderConfig(services backend.ServiceRegistrations) (string, error) {
	c := haproxy.NewConfig()
	c.Section("global").Add(globalOptions...)
	c.Section("defaults").Add(defaultsOptions...)

	// Create stats section
	if s.StatsPort != 0 && s.StatsUser != "" && s.StatsPassword != "" {
		statsSection := c.Section("frontend stats")
		statsSsl := ""
		if s.StatsSslCert != "" {
			statsSsl = fmt.Sprintf("ssl crt %s no-sslv3", filepath.Join(s.SslCertsFolder, s.StatsSslCert))
		}
		statsSection.Add(
			fmt.Sprintf("bind *:%d %s", s.StatsPort, statsSsl),
			"stats enable",
			"stats uri /",
			"stats realm Haproxy\\ Statistics",
			fmt.Sprintf("stats auth %s:%s", s.StatsUser, s.StatsPassword),
		)
	}

	// Create user lists for each frontend (that needs it)
	for _, sr := range services {
		for selIndex, sel := range sr.Selectors {
			if len(sel.Users) == 0 {
				continue
			}
			userListSection := c.Section("userlist " + userListName(sr, selIndex))
			for _, user := range sel.Users {
				userListSection.Add(fmt.Sprintf("user %s password %s", user.Name, user.PasswordHash))
			}
		}
	}

	// Create config for all registrations
	publicFrontEndSection := c.Section("frontend http-in")
	publicFrontEndSection.Add("bind *:80")
	// Collect certificates
	certs := []string{}
	certsSet := make(map[string]struct{})
	for _, sr := range services {
		for _, sel := range sr.Selectors {
			if !sel.Private && sel.IsSecure() {
				certPath := sel.TmpSslCertPath
				if certPath == "" {
					certPath = filepath.Join(s.SslCertsFolder, sel.SslCertName)
				}
				if _, ok := certsSet[certPath]; !ok {
					crt := fmt.Sprintf("crt %s", certPath)
					certs = append(certs, crt)
					certsSet[certPath] = struct{}{}
				}
			}
		}
	}
	if len(certs) > 0 {
		publicFrontEndSection.Add(
			fmt.Sprintf("bind *:443 ssl %s no-sslv3", strings.Join(certs, " ")),
		)
	}
	if s.ForceSsl {
		publicFrontEndSection.Add("redirect scheme https if !{ ssl_fc }")
	}
	publicFrontEndSection.Add(
		"reqadd X-Forwarded-Port:\\ %[dst_port]",
		"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
		"default_backend fallback",
	)
	aclNameGen := NewNameGenerator("acl")
	// Create acls
	publicFrontEndSelection := selection{Private: false, Http: true}
	useBlocks := createAcls(publicFrontEndSection, services, publicFrontEndSelection, aclNameGen)
	// Create link to backend
	createUseBackends(publicFrontEndSection, useBlocks, publicFrontEndSelection)

	// Create config for private HTTP services
	privateFrontEndSection := c.Section("frontend http-in-private")
	privateFrontEndSection.Add("bind *:81")
	privateFrontEndSection.Add(
		"reqadd X-Forwarded-Port:\\ %[dst_port]",
		"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
		"default_backend fallback",
	)
	// Create acls
	privateFrontEndSelection := selection{Private: true, Http: true}
	useBlocks = createAcls(privateFrontEndSection, services, privateFrontEndSelection, aclNameGen)
	// Create link to backend
	createUseBackends(privateFrontEndSection, useBlocks, privateFrontEndSelection)

	// Create config for private TCP services
	if s.PrivateTcpSslCert != "" {
		privateTcpFrontEndSection := c.Section("frontend tcp-in-private")
		privateTcpSsl := fmt.Sprintf("ssl crt %s generate-certificates ca-sign-file %s no-sslv3",
			filepath.Join(s.SslCertsFolder, s.PrivateTcpSslCert),
			filepath.Join(s.SslCertsFolder, s.PrivateTcpSslCert),
		)
		privateTcpFrontEndSection.Add("bind *:82 " + privateTcpSsl)
		privateTcpFrontEndSection.Add(
			"mode tcp",
			"default_backend fallback",
		)
		// Create acls
		privateTcpFrontEndSelection := selection{Private: true, Http: false}
		useBlocks = createAcls(privateTcpFrontEndSection, services, privateTcpFrontEndSelection, aclNameGen)
		// Create link to backend
		createUseBackends(privateTcpFrontEndSection, useBlocks, privateTcpFrontEndSelection)
	}

	// Create backends
	for _, sr := range services {
		for _, private := range []bool{false, true} {
			if private {
				if !sr.HasPrivateSelectors() {
					continue
				}
			} else {
				if !sr.HasPublicSelectors() {
					continue
				}
			}
			// Create backend
			backendSection := c.Section(fmt.Sprintf("backend %s", backendName(sr, selection{Private: private, Http: sr.IsHttp()})))
			backendSection.Add(
				"balance roundrobin",
			)
			if sr.IsHttp() {
				backendSection.Add("mode http")
			} else if sr.IsTcp() {
				backendSection.Add("mode tcp")
			} else {
				return "", maskAny(fmt.Errorf("Unknown service mode '%s'", sr.Mode))
			}
			if sr.HttpCheckPath != "" {
				backendSection.Add(fmt.Sprintf("option httpchk GET %s", sr.HttpCheckPath))
			}

			for i, instance := range sr.Instances {
				id := fmt.Sprintf("%s-%d-%d", sr.ServiceName, sr.ServicePort, i)
				check := ""
				if sr.HttpCheckPath != "" {
					check = "check"
				}
				backendSection.Add(fmt.Sprintf("server %s %s:%d %s", id, instance.IP, instance.Port, check))
			}
		}
	}

	// Create fallback backend
	fbbSection := c.Section("backend fallback")
	fbbSection.Add(
		"mode http",
		"balance roundrobin",
		"errorfile 503 /app/errors/404.http", // Force not found
	)

	// Render config
	return c.Render(), nil
}

// createAclRules create `acl` rules for the given selector
func createAclRules(sel backend.ServiceSelector) []string {
	result := []string{}
	if sel.Domain != "" {
		if sel.IsSecure() {
			result = append(result, fmt.Sprintf("ssl_fc_sni -i %s", sel.Domain))
		} else {
			result = append(result, fmt.Sprintf("hdr_dom(host) -i %s", sel.Domain))
		}
	}
	if sel.PathPrefix != "" {
		result = append(result, fmt.Sprintf("path_beg %s", sel.PathPrefix))
	}
	return result
}

// creteAcls create `acl` rules for the given services and adds them
// to the given section
func createAcls(section *haproxy.Section, services backend.ServiceRegistrations, selection selection, ng *nameGenerator) []useBlock {
	pairs := selectorServicePairs{}
	for _, sr := range services {
		if sr.IsHttp() == selection.Http {
			for selIndex, sel := range sr.Selectors {
				if sel.Private == selection.Private {
					pairs = append(pairs, selectorServicePair{
						Selector:      sel,
						SelectorIndex: selIndex,
						Service:       sr,
					})
				}
			}
		}
	}
	sort.Sort(pairs)

	useBlocks := []useBlock{}
	for _, pair := range pairs {
		rules := createAclRules(pair.Selector)

		authAclName := ""
		if len(pair.Selector.Users) > 0 {
			authAclName = "auth_" + ng.Next()
			section.Add(fmt.Sprintf("acl %s http_auth(%s)", authAclName, userListName(pair.Service, pair.SelectorIndex)))
		}

		if len(rules) == 0 && authAclName == "" {
			continue
		}
		aclNames := []string{}
		for _, rule := range rules {
			aclName := ng.Next()
			section.Add(fmt.Sprintf("acl %s %s", aclName, rule))
			aclNames = append(aclNames, aclName)
		}
		useBlocks = append(useBlocks, useBlock{
			AclNames:    aclNames,
			AuthAclName: authAclName,
			Service:     pair.Service,
		})
	}
	return useBlocks
}

// createUseBackends creates a `use_backend` rules for the given input
// and adds it to the given section
func createUseBackends(section *haproxy.Section, useBlocks []useBlock, selection selection) {
	for _, useBlock := range useBlocks {
		if len(useBlock.AclNames) == 0 {
			continue
		}
		acls := strings.Join(useBlock.AclNames, " ")
		if useBlock.AuthAclName != "" {
			section.Add(fmt.Sprintf("http-request allow if %s %s", acls, useBlock.AuthAclName))
			section.Add(fmt.Sprintf("http-request auth if %s !%s", acls, useBlock.AuthAclName))
		}
		section.Add(fmt.Sprintf("use_backend %s if %s", backendName(useBlock.Service, selection), acls))
	}
}

// backendName creates a valid name for the backend of this registration
// in haproxy.
func backendName(sr backend.ServiceRegistration, selection selection) string {
	return fmt.Sprintf("backend_%s_%d_%s", cleanName(sr.ServiceName), sr.ServicePort, visibilityPostfix(selection))
}

// userListName creates a valid name for the userlist of this registration
// in haproxy.
func userListName(sr backend.ServiceRegistration, selectorIndex int) string {
	return fmt.Sprintf("userlist_%s_%d_%d", cleanName(sr.ServiceName), sr.ServicePort, selectorIndex)
}

// cleanName removes invalid characters (for haproxy conf) from the given name
func cleanName(s string) string {
	return s // TODO
}

func visibilityPostfix(selection selection) string {
	if selection.Private {
		if selection.Http {
			return "private"
		} else {
			return "private-tcp"
		}
	}
	return "public"
}

type selectorServicePair struct {
	Selector      backend.ServiceSelector
	SelectorIndex int
	Service       backend.ServiceRegistration
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
