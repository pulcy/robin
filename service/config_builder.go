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

const (
	PublicHttpPort    = 80
	PublicHttpsPort   = 443
	PrivateHttpPort   = 81
	PrivateTcpSslPort = 82
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
	AllowInsecure     bool
	RewriteRules      []backend.RewriteRule
}

type frontend struct {
	index  int // Used for sorting only
	Port   int
	Public bool
	Mode   string
}

func (f frontend) Name() string {
	if f.Public {
		return fmt.Sprintf("public_%s_in_%d", f.Mode, f.Port)
	}
	return fmt.Sprintf("private_%s_in_%d", f.Mode, f.Port)
}

// IsHTTP returns true if Mode == "http"
func (f frontend) IsHTTP() bool {
	return f.Mode == "http"
}

// IsTCP returns true if Mode == "tcp"
func (f frontend) IsTCP() bool {
	return f.Mode == "tcp"
}

type frontendList []frontend

func (l frontendList) Len() int { return len(l) }

// Less reports whether the element with
// index i should sort before the element with index j.
func (l frontendList) Less(i, j int) bool {
	if l[i].index < l[j].index {
		return true
	}
	if l[i].index > l[j].index {
		return false
	}
	return l[i].Name() < l[j].Name()
}

// Swap swaps the elements with indexes i and j.
func (l frontendList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
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
		if sr.Public {
			for _, sel := range sr.Selectors {
				if sel.IsSecure() {
					certPath := sel.TmpSslCertPath
					if certPath == "" {
						certPath = filepath.Join(s.SslCertsFolder, sel.SslCertName)
					}
					certFolder := filepath.Dir(certPath)
					if _, ok := certsSet[certFolder]; !ok {
						crt := fmt.Sprintf("crt %s", certFolder)
						certs = append(certs, crt)
						certsSet[certFolder] = struct{}{}
					}
				}
			}
		}
	}

	// Collect frontends
	var frontends frontendList
	frontendMap := make(map[string]frontend)
	collectFrontend := func(index, edgePort int, public bool, mode string) {
		if (public && s.ExcludePublic) || (!public && s.ExcludePrivate) {
			return // Exclude
		}
		f := frontend{
			index:  index,
			Port:   edgePort,
			Public: public,
			Mode:   mode,
		}
		if _, ok := frontendMap[f.Name()]; !ok {
			frontendMap[f.Name()] = f
			frontends = append(frontends, f)
		}
	}
	collectFrontend(0, PublicHttpPort, true, "http")   // Always create a public HTTP frontend
	collectFrontend(1, PrivateHttpPort, false, "http") // Always create a private HTTP frontend
	for _, sr := range services {
		collectFrontend(2, sr.EdgePort, sr.Public, sr.Mode)
	}
	sort.Sort(frontends)

	// Create all frontends
	aclNameGen := NewNameGenerator("acl")
	backends := make(map[string]backendConfig)
	for _, frontend := range frontends {
		frontendSection := c.Section(fmt.Sprintf("frontend %s", frontend.Name()))
		host := "*"
		if frontend.Public {
			if s.PublicHost != "" {
				host = s.PublicHost
			}
		} else {
			if s.PrivateHost != "" {
				host = s.PrivateHost
			}
		}
		bind := fmt.Sprintf("bind %s:%d", host, frontend.Port)
		if !frontend.Public && frontend.IsTCP() && frontend.Port == PrivateTcpSslPort && s.PrivateTcpSslCert != "" {
			bind = fmt.Sprintf("%s ssl generate-certificates ca-sign-file %s crt %s no-sslv3",
				bind,
				filepath.Join(s.SslCertsFolder, s.PrivateTcpSslCert),
				filepath.Join(s.SslCertsFolder, s.PrivateTcpSslCert),
			)
		}
		frontendSection.Add(bind)
		var secureFrontendSection *haproxy.Section
		frontendSections := []*haproxy.Section{frontendSection}
		if frontend.Public && frontend.Port == PublicHttpPort && frontend.IsHTTP() && len(certs) > 0 {
			secureFrontendSection = c.Section(fmt.Sprintf("frontend secure-%s", frontend.Name()))
			frontendSections = append(frontendSections, secureFrontendSection)
			secureFrontendSection.Add(fmt.Sprintf("bind %s:%d ssl %s no-sslv3", host, PublicHttpsPort, strings.Join(certs, " ")))
		}
		for _, section := range frontendSections {
			section.Add(fmt.Sprintf("mode %s", frontend.Mode))
			if frontend.IsHTTP() {
				section.Add(
					"option forwardfor",
					//"option httplog",
					"reqadd X-Forwarded-Port:\\ %[dst_port]",
					"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
				)
			}
			section.Add("default_backend fallback")
		}
		// Create acls
		var useBlocks []useBlock
		isHTTPS := false
		useBlocks, backends = createAcls(frontendSection, services, frontend, isHTTPS, aclNameGen, backends)
		// Create link to backends
		createUseBackends(frontendSection, useBlocks, frontend, (secureFrontendSection != nil), frontend.Public && frontend.IsHTTP() && s.ForceSsl)
		if secureFrontendSection != nil {
			isHTTPS = true
			useBlocks, backends = createAcls(secureFrontendSection, services, frontend, isHTTPS, aclNameGen, backends)
			createUseBackends(secureFrontendSection, useBlocks, frontend, false, false)
		}
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

	// Private stats
	if s.PrivateStatsPort != 0 {
		privateStatsSection := c.Section("frontend private-stats")
		privateStatsSection.Add(
			"mode http",
			fmt.Sprintf("bind 127.0.0.1:%d", s.PrivateStatsPort),
			"stats enable",
			"stats uri /",
		)
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
				if sr.HttpCheckPath != "" || sr.HttpCheckMethod != "" || sr.Backup {
					check = "check"
					if sr.Backup {
						check = check + " backup"
					}
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
func createAclRules(sel backend.ServiceSelector, isHttps, isTcp bool) []string {
	result := []string{}
	if sel.Domain != "" {
		if (sel.IsSecure() && isHttps) || isTcp {
			result = append(result, fmt.Sprintf("ssl_fc_sni -i %s", sel.Domain))
		} else {
			result = append(result, fmt.Sprintf("hdr_dom(host) -i %s", sel.Domain))
		}
	}
	if sel.PathPrefix != "" {
		result = append(result, fmt.Sprintf("path_beg %s", sel.PathPrefix))
	}
	if len(result) == 0 && isTcp {
		result = append(result, "always_true")
	}
	return result
}

// creteAcls create `acl` rules for the given services and adds them
// to the given section
func createAcls(section *haproxy.Section, services backend.ServiceRegistrations, selection frontend, isHttps bool, ng *nameGenerator, backends map[string]backendConfig) ([]useBlock, map[string]backendConfig) {
	pairs := selectorServicePairs{}
	for _, sr := range services {
		if sr.IsHttp() == selection.IsHTTP() && sr.Public == selection.Public {
			for selIndex, sel := range sr.Selectors {
				pairs = append(pairs, selectorServicePair{
					Selector:      sel,
					SelectorIndex: selIndex,
					Service:       sr,
				})
			}
		}
	}
	sort.Sort(pairs)

	useBlocks := []useBlock{}
	rules2Block := make(map[string]useBlock)
	for _, pair := range pairs {
		rules := createAclRules(pair.Selector, isHttps, pair.Service.IsTcp())

		authAclName := ""
		if len(pair.Selector.Users) > 0 {
			authAclName = "auth_" + ng.Next()
			section.Add(fmt.Sprintf("acl %s http_auth(%s)", authAclName, userListName(pair.Service, pair.SelectorIndex)))
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
				section.Add(fmt.Sprintf("acl %s %s", aclName, rule))
				aclNames = append(aclNames, aclName)
			}
			backendName := generateBackendName(pair.Service, selection)
			block = useBlock{
				BackendName:       backendName,
				AclNames:          aclNames,
				AuthAclName:       authAclName,
				RewriteRules:      pair.Selector.RewriteRules,
				AllowUnauthorized: pair.Selector.AllowUnauthorized,
				AllowInsecure:     pair.Selector.AllowInsecure,
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
func createUseBackends(section *haproxy.Section, useBlocks []useBlock, selection frontend, redirectHttps, forceSecure bool) {
	for _, useBlock := range useBlocks {
		if len(useBlock.AclNames) == 0 {
			continue
		}
		acls := strings.Join(useBlock.AclNames, " ")
		skipUseBackend := false
		if !useBlock.AllowInsecure && forceSecure {
			section.Add(fmt.Sprintf("redirect scheme https if !{ ssl_fc } %s", acls))
			skipUseBackend = true
		} else if useBlock.AllowUnauthorized {
			section.Add(fmt.Sprintf("http-request allow if %s", acls))
		} else if useBlock.AuthAclName != "" {
			if redirectHttps {
				section.Add(fmt.Sprintf("redirect scheme https if !{ ssl_fc } %s", acls))
			} else {
				section.Add(fmt.Sprintf("http-request allow if %s %s", acls, useBlock.AuthAclName))
				section.Add(fmt.Sprintf("http-request auth if %s !%s", acls, useBlock.AuthAclName))
			}
		}
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
func generateBackendName(sr backend.ServiceRegistration, selection frontend) string {
	return fmt.Sprintf("backend_%s_%d_%s", cleanName(sr.ServiceName), sr.ServicePort, selection.Name())
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
