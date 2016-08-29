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

	"github.com/pulcy/robin/haproxy"
	"github.com/pulcy/robin/service/backend"
)

var (
	globalOptions = []string{
		//"log global",
		"quiet",
		"tune.ssl.default-dh-param 2048",
		"ssl-default-bind-ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-DSS-AES128-GCM-SHA256:kEDH+AESGCM:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA:ECDHE-ECDSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-DSS-AES128-SHA256:DHE-RSA-AES256-SHA256:DHE-DSS-AES256-SHA:DHE-RSA-AES256-SHA:AES128-GCM-SHA256:AES256-GCM-SHA384:AES128:AES256:AES:CAMELLIA:!aNULL:!eNULL:!EXPORT:!DES:!RC4:!MD5:!PSK:!aECDH:!EDH-DSS-DES-CBC3-SHA:!EDH-RSA-DES-CBC3-SHA:!KRB5-DES-CBC3-SHA",
	}
	defaultsOptions = []string{
		"mode tcp",
		"timeout connect 5000ms",
		"timeout client 50000ms",
		"timeout server 50000ms",
		"option http-server-close",
		//"log global",
		//"option dontlognull",
		"errorfile 400 /app/errors/400.http",
		"errorfile 403 /app/errors/403.http",
		"errorfile 408 /app/errors/408.http",
		"errorfile 500 /app/errors/500.http",
		"errorfile 502 /app/errors/502.http",
		"errorfile 503 /app/errors/503.http",
		"errorfile 504 /app/errors/504.http",
	}
	securityOptions = []string{
		"http-response set-header Strict-Transport-Security max-age=63072000",
		"http-response set-header X-Frame-Option SAMEORIGIN",
		"http-response set-header X-XSS-Protection 1;mode=block",
		"http-response set-header X-Content-Type-Options nosniff",
	}
)

type useBlock struct {
	BackendName       string
	AclNames          []string
	AuthAclName       string
	AllowUnauthorized bool
	RewriteRules      []backend.RewriteRule
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

	// Create config for all registrations
	publicHttpFrontEndSection := c.Section("frontend http-in")
	publicHttpFrontEndSection.Add("bind *:80")
	var publicHttpsFrontEndSection *haproxy.Section
	publicFrontEndSections := []*haproxy.Section{publicHttpFrontEndSection}
	if len(certs) > 0 {
		publicHttpsFrontEndSection = c.Section("frontend https-in")
		publicFrontEndSections = append(publicFrontEndSections, publicHttpsFrontEndSection)
		publicHttpsFrontEndSection.Add(
			fmt.Sprintf("bind *:443 ssl %s no-sslv3", strings.Join(certs, " ")),
		)
		if s.ForceSsl {
			publicHttpFrontEndSection.Add("redirect scheme https if !{ ssl_fc }")
		}
	}
	for _, section := range publicFrontEndSections {
		section.Add(
			"mode http",
			"option forwardfor",
			//"option httplog",
			"reqadd X-Forwarded-Port:\\ %[dst_port]",
			"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
			"default_backend fallback",
		)
	}
	aclNameGen := NewNameGenerator("acl")
	backends := make(map[string]backendConfig)
	// Create acls
	publicFrontEndSelection := selection{Private: false, Http: true}
	useBlocks, backends := createAcls(publicFrontEndSections, services, publicFrontEndSelection, aclNameGen, backends)
	// Create link to backends
	createUseBackends(publicHttpFrontEndSection, useBlocks, publicFrontEndSelection, (publicHttpsFrontEndSection != nil))
	if publicHttpsFrontEndSection != nil {
		createUseBackends(publicHttpsFrontEndSection, useBlocks, publicFrontEndSelection, false)
	}

	// Create config for private HTTP services
	privateFrontEndSection := c.Section("frontend http-in-private")
	privateFrontEndSection.Add("bind *:81")
	privateFrontEndSection.Add(
		"mode http",
		"option forwardfor",
		//"option httplog",
		"reqadd X-Forwarded-Port:\\ %[dst_port]",
		"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
		"default_backend fallback",
	)
	// Create acls
	privateFrontEndSelection := selection{Private: true, Http: true}
	useBlocks, backends = createAcls([]*haproxy.Section{privateFrontEndSection}, services, privateFrontEndSelection, aclNameGen, backends)
	// Create link to backend
	createUseBackends(privateFrontEndSection, useBlocks, privateFrontEndSelection, false)

	// Create config for private TCP services
	if s.PrivateTcpSslCert != "" {
		privateTcpFrontEndSection := c.Section("frontend tcp-in-private")
		privateTcpSsl := fmt.Sprintf("ssl generate-certificates ca-sign-file %s crt %s no-sslv3",
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
		useBlocks, backends = createAcls([]*haproxy.Section{privateTcpFrontEndSection}, services, privateTcpFrontEndSelection, aclNameGen, backends)
		// Create link to backend
		createUseBackends(privateTcpFrontEndSection, useBlocks, privateTcpFrontEndSelection, false)
	}

	// Create stats section
	if s.StatsPort != 0 && s.StatsUser != "" && s.StatsPassword != "" {
		statsSection := c.Section("frontend stats")
		statsCerts := strings.Join(certs, " ")
		if s.StatsSslCert != "" {
			statsCerts = fmt.Sprintf("crt %s %s", filepath.Join(s.SslCertsFolder, s.StatsSslCert), statsCerts)
		}
		statsSsl := ""
		if statsCerts != "" {
			statsSsl = fmt.Sprintf("ssl %s no-sslv3", statsCerts)
		}
		statsSection.Add(
			"mode http",
			fmt.Sprintf("bind *:%d %s", s.StatsPort, statsSsl),
			"stats enable",
			"stats uri /",
			"stats realm Haproxy\\ Statistics",
			fmt.Sprintf("stats auth %s:%s", s.StatsUser, s.StatsPassword),
		)
		if statsCerts != "" {
			statsSection.Add(securityOptions...)
		}
	}

	// Create backends
	backendNames := []string{}
	for name, _ := range backends {
		backendNames = append(backendNames, name)
	}
	sort.Strings(backendNames)
	for _, name := range backendNames {
		// Create backend
		b := backends[name]
		backendSection := c.Section(fmt.Sprintf("backend %s", b.Name))
		sticky, err := b.IsSticky()
		if err != nil {
			return "", maskAny(err)
		}
		if sticky {
			backendSection.Add("balance source")
		} else {
			backendSection.Add("balance roundrobin")
		}
		mode, err := b.Mode()
		if err != nil {
			return "", maskAny(err)
		}
		if mode == "http" {
			backendSection.Add("mode http")
			if !b.HasAllowUnauthorized() {
				backendSection.Add(securityOptions...)
			}
		} else if mode == "tcp" {
			backendSection.Add("mode tcp")
		} else {
			return "", maskAny(fmt.Errorf("Unknown service mode '%s'", mode))
		}
		method, hasCheckMethod, err := b.HttpCheckMethod()
		if err != nil {
			return "", maskAny(err)
		}
		path, hasCheckPath, err := b.HttpCheckPath()
		if err != nil {
			return "", maskAny(err)
		}
		if hasCheckMethod || hasCheckPath {
			backendSection.Add(fmt.Sprintf("option httpchk %s %s", method, path))
		}
		for _, sr := range b.Services {
			for i, instance := range sr.Instances {
				id := fmt.Sprintf("s%d-%s-%d", i, instance.IP, instance.Port)
				id = strings.Replace(id, ".", "_", -1)
				id = strings.Replace(id, ":", "_", -1)
				id = strings.Replace(id, "[", "", -1)
				id = strings.Replace(id, "]", "", -1)
				id = strings.Replace(id, "%", "", -1)
				check := ""
				if sr.HttpCheckPath != "" || sr.HttpCheckMethod != "" {
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
func createAclRules(sel backend.ServiceSelector, isTcp bool) []string {
	result := []string{}
	if sel.Domain != "" {
		if sel.IsSecure() || isTcp {
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
func createAcls(sections []*haproxy.Section, services backend.ServiceRegistrations, selection selection, ng *nameGenerator, backends map[string]backendConfig) ([]useBlock, map[string]backendConfig) {
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
	rules2Block := make(map[string]useBlock)
	for _, pair := range pairs {
		rules := createAclRules(pair.Selector, pair.Service.IsTcp())

		authAclName := ""
		if len(pair.Selector.Users) > 0 {
			authAclName = "auth_" + ng.Next()
			for _, section := range sections {
				section.Add(fmt.Sprintf("acl %s http_auth(%s)", authAclName, userListName(pair.Service, pair.SelectorIndex)))
			}
		}

		if len(rules) == 0 && authAclName == "" {
			continue
		}
		rulesKey := strings.Join(rules, ",")
		block, ok := rules2Block[rulesKey]
		if !ok {
			aclNames := []string{}
			for _, rule := range rules {
				aclName := ng.Next()
				for _, section := range sections {
					section.Add(fmt.Sprintf("acl %s %s", aclName, rule))
				}
				aclNames = append(aclNames, aclName)
			}
			backendName := generateBackendName(pair.Service, selection)
			block = useBlock{
				BackendName:       backendName,
				AclNames:          aclNames,
				AuthAclName:       authAclName,
				RewriteRules:      pair.Selector.RewriteRules,
				AllowUnauthorized: pair.Selector.AllowUnauthorized,
			}
			useBlocks = append(useBlocks, block)
			rules2Block[rulesKey] = block
		}
		backendCfg, ok := backends[block.BackendName]
		if !ok {
			backendCfg = backendConfig{
				Name: block.BackendName,
			}
		}
		if !backendCfg.Services.Contains(pair.Service) {
			backendCfg.Services = append(backendCfg.Services, pair.Service)
		}
		backends[block.BackendName] = backendCfg
	}
	return useBlocks, backends
}

// createUseBackends creates a `use_backend` rules for the given input
// and adds it to the given section
func createUseBackends(section *haproxy.Section, useBlocks []useBlock, selection selection, redirectHttps bool) {
	for _, useBlock := range useBlocks {
		if len(useBlock.AclNames) == 0 {
			continue
		}
		acls := strings.Join(useBlock.AclNames, " ")
		if useBlock.AllowUnauthorized {
			section.Add(fmt.Sprintf("http-request allow if %s", acls))
		} else if useBlock.AuthAclName != "" {
			if redirectHttps {
				section.Add(fmt.Sprintf("redirect scheme https if !{ ssl_fc } %s", acls))
			} else {
				section.Add(fmt.Sprintf("http-request allow if %s %s", acls, useBlock.AuthAclName))
				section.Add(fmt.Sprintf("http-request auth if %s !%s", acls, useBlock.AuthAclName))
			}
		}
		skipUseBackend := false
		for _, rwRule := range useBlock.RewriteRules {
			if rwRule.PathPrefix != "" {
				prefix := strings.TrimSuffix(rwRule.PathPrefix, "/")
				section.Add(fmt.Sprintf("http-request set-path %s%s if %s", prefix, "%[path]", acls))
			}
			if rwRule.RemovePathPrefix != "" {
				prefix := strings.TrimPrefix(strings.TrimSuffix(rwRule.RemovePathPrefix, "/"), "/")
				section.Add(fmt.Sprintf(`reqrep ^([^\ :]*)\ /%s/(.*)     \1\ /\2  if %s`, prefix, acls))
			}
			if rwRule.Domain != "" {
				if redirectHttps {
					section.Add(fmt.Sprintf("http-request redirect prefix https://%s code 301 if %s", rwRule.Domain, acls))
				} else {
					section.Add(fmt.Sprintf("http-request redirect prefix https://%s code 301 if { ssl_fc } %s", rwRule.Domain, acls))
					section.Add(fmt.Sprintf("http-request redirect prefix http://%s code 301 if !{ ssl_fc } %s", rwRule.Domain, acls))
				}
				skipUseBackend = true
			}
		}
		if !skipUseBackend {
			section.Add(fmt.Sprintf("use_backend %s if %s", useBlock.BackendName, acls))
		}
	}
}

// generateBackendName creates a valid name for the backend of this registration
// in haproxy.
func generateBackendName(sr backend.ServiceRegistration, selection selection) string {
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
