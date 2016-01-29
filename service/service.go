package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/op/go-logging"

	"arvika.pulcy.com/pulcy/load-balancer/haproxy"
)

const (
	osExitDelay  = time.Second * 3
	confPerm     = os.FileMode(0664) // rw-rw-r
	refreshDelay = time.Second * 5
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

type ServiceConfig struct {
	HaproxyConfPath string
	HaproxyPath     string
	HaproxyPidPath  string
	StatsPort       int
	StatsUser       string
	StatsPassword   string
	StatsSslCert    string
	SslCertsFolder  string
	ForceSsl        bool
	PrivateHost     string
}

type ServiceDependencies struct {
	Logger  *logging.Logger
	Backend Backend
}

type Service struct {
	ServiceConfig
	ServiceDependencies

	signalCounter uint32
	lastConfig    string
	lastPid       int
	dirty         bool
}

// NewService creates a new service instance.
func NewService(config ServiceConfig, deps ServiceDependencies) *Service {
	if config.HaproxyPath == "" {
		config.HaproxyPath = "haproxy"
	}
	if config.HaproxyPidPath == "" {
		config.HaproxyPidPath = "/var/run/haproxy.pid"
	}
	return &Service{
		ServiceConfig:       config,
		ServiceDependencies: deps,
	}
}

// Run starts the service and waits for OS signals to terminate it.
func (s *Service) Run() {
	go s.backendMonitorLoop()
	go s.configLoop()
	go func() {
		time.Sleep(time.Second)
		s.dirty = true
	}()
	s.listenSignals()
}

// configLoop updates the haproxy config, and then waits
// for changes in the backend.
func (s *Service) configLoop() {
	for {
		if s.dirty {
			if err := s.updateHaproxy(); err != nil {
				s.Logger.Error("Failed to update haproxy: %#v", err)
			} else {
				s.dirty = false
			}
		}
		select {
		case <-time.After(refreshDelay):
		}
	}
}

// backendMonitorLoop monitors the configuration backend for changes.
// When it detects a change, it set a dirty flag.
func (s *Service) backendMonitorLoop() {
	for {
		if err := s.Backend.Watch(); err != nil {
			s.Logger.Error("Failed to watch for backend changes: %#v", err)
		}
		s.dirty = true
	}
}

// update the haproxy configuration
func (s *Service) updateHaproxy() error {
	// Create a new config (in temp path)
	config, tempConf, err := s.createConfig()
	if err != nil {
		return maskAny(err)
	}

	// Cleanup afterwards
	defer os.Remove(tempConf)

	// If nothing has changed, don't do anything
	if s.lastConfig == config {
		s.Logger.Debug("Config has not changed")
		return nil
	}

	// Validate the config
	if err := s.validateConfig(tempConf); err != nil {
		s.Logger.Error("haproxy config validation failed: %#v", err)
		return maskAny(err)
	}

	// Move config to correct place
	os.Remove(s.HaproxyConfPath)
	if err := ioutil.WriteFile(s.HaproxyConfPath, []byte(config), confPerm); err != nil {
		s.Logger.Error("Cannot copy haproxy config to %s: %#v", s.HaproxyConfPath, err)
		return maskAny(err)
	}

	// Restart haproxy
	if err := s.restartHaproxy(); err != nil {
		return maskAny(err)
	}

	// Rember the current config
	s.lastConfig = config

	s.Logger.Info("Restarted haproxy")

	return nil
}

type useBlock struct {
	AclNames    []string
	AuthAclName string
	Service     ServiceRegistration
}

// createConfig creates a new haproxy configuration file.
// It returns the path of the new config file.
func (s *Service) createConfig() (string, string, error) {
	c := haproxy.NewConfig()
	c.Section("global").Add(globalOptions...)
	c.Section("defaults").Add(defaultsOptions...)

	// Fetch data from backend
	services, err := s.Backend.Services()
	if err != nil {
		return "", "", maskAny(err)
	}
	services.Sort()

	// Log services
	s.Logger.Info("Found %d services", len(services))
	for _, srv := range services {
		s.Logger.Info("Service: %#v", srv)
	}

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
			userListSection := c.Section("userlist " + sr.userListName(selIndex))
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
			if !sel.Private && sel.SslCert != "" {
				if _, ok := certsSet[sel.SslCert]; !ok {
					crt := fmt.Sprintf("crt %s", filepath.Join(s.SslCertsFolder, sel.SslCert))
					certs = append(certs, crt)
					certsSet[sel.SslCert] = struct{}{}
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
	useBlocks := createAcls(publicFrontEndSection, services, false, aclNameGen)
	// Create link to backend
	createUseBackends(publicFrontEndSection, useBlocks, false)

	// Create config for private services
	privateFrontEndSection := c.Section("frontend http-in-private")
	privateFrontEndSection.Add("bind *:81")
	privateFrontEndSection.Add(
		"reqadd X-Forwarded-Port:\\ %[dst_port]",
		"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
		"default_backend fallback",
	)
	// Create acls
	useBlocks = createAcls(privateFrontEndSection, services, true, aclNameGen)
	// Create link to backend
	createUseBackends(privateFrontEndSection, useBlocks, true)

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
			backendSection := c.Section(fmt.Sprintf("backend %s", sr.backendName(private)))
			backendSection.Add(
				"mode http",
				"balance roundrobin",
			)
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
	config := c.Render()

	// Create temp file first
	tempFile, err := ioutil.TempFile("", "haproxy")
	if err != nil {
		return "", "", maskAny(err)
	}
	defer tempFile.Close()
	if _, err := tempFile.WriteString(config); err != nil {
		return "", "", maskAny(err)
	}
	return config, tempFile.Name(), nil
}

// createAclRules create `acl` rules for the given selector
func createAclRules(sel ServiceSelector) []string {
	result := []string{}
	if sel.Domain != "" {
		if sel.SslCert != "" {
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
func createAcls(section *haproxy.Section, services ServiceRegistrations, private bool, ng *nameGenerator) []useBlock {
	pairs := selectorServicePairs{}
	for _, sr := range services {
		for selIndex, sel := range sr.Selectors {
			if sel.Private == private {
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
	for _, pair := range pairs {
		rules := createAclRules(pair.Selector)

		authAclName := ""
		if len(pair.Selector.Users) > 0 {
			authAclName = "auth_" + ng.Next()
			section.Add(fmt.Sprintf("acl %s http_auth(%s)", authAclName, pair.Service.userListName(pair.SelectorIndex)))
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
func createUseBackends(section *haproxy.Section, useBlocks []useBlock, private bool) {
	for _, useBlock := range useBlocks {
		if len(useBlock.AclNames) == 0 {
			continue
		}
		acls := strings.Join(useBlock.AclNames, " ")
		if useBlock.AuthAclName != "" {
			section.Add(fmt.Sprintf("http-request allow if %s %s", acls, useBlock.AuthAclName))
			section.Add(fmt.Sprintf("http-request auth if %s !%s", acls, useBlock.AuthAclName))
		}
		section.Add(fmt.Sprintf("use_backend %s if %s", useBlock.Service.backendName(private), acls))
	}
}

// validateConfig calls haproxy to validate the given config file.
func (s *Service) validateConfig(confPath string) error {
	cmd := exec.Command(s.HaproxyPath, "-c", "-f", confPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.Logger.Error("Error in haproxy config: %s", string(output))
		return maskAny(err)
	}
	return nil
}

// restartHaproxy restarts haproxy, killing previous instances
func (s *Service) restartHaproxy() error {
	args := []string{
		"-f",
		s.HaproxyConfPath,
	}
	if s.lastPid > 0 {
		args = append(args, "-sf", strconv.Itoa(s.lastPid))
	}

	s.Logger.Debug("Starting haproxy with %#v", args)
	cmd := exec.Command(s.HaproxyPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGUSR1}
	cmd.Stdin = bytes.NewReader([]byte{})
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		s.Logger.Error("Failed to start haproxy: %#v", err)
		return maskAny(err)
	}

	pid := -1
	proc := cmd.Process
	if proc != nil {
		pid = proc.Pid
	}
	s.lastPid = pid
	s.Logger.Debug("haxproxy pid %d started", pid)

	go func() {
		// Wait for haproxy to terminate so we avoid defunct processes
		if err := cmd.Wait(); err != nil {
			s.Logger.Error("haproxy pid %d wait returned an error: %#v", pid, err)
		} else {
			s.Logger.Debug("haproxy pid %d terminated", pid)
		}
	}()

	return nil
}

// close closes this service in a timely manor.
func (s *Service) close() {
	// Interrupt the process when closing is requested twice.
	if atomic.AddUint32(&s.signalCounter, 1) >= 2 {
		s.exitProcess()
	}

	s.Logger.Info("shutting down server in %s", osExitDelay.String())
	time.Sleep(osExitDelay)

	s.exitProcess()
}

// exitProcess terminates this process with exit code 1.
func (s *Service) exitProcess() {
	s.Logger.Info("shutting down server")
	os.Exit(0)
}

// listenSignals waits for incoming OS signals and acts upon them
func (s *Service) listenSignals() {
	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// Block until a signal is received.
	for {
		select {
		case sig := <-c:
			s.Logger.Info("server received signal %s", sig)
			go s.close()
		}
	}
}
