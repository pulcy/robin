package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/op/go-logging"

	"arvika.pulcy.com/pulcy/load-balancer/haproxy"
)

const (
	osExitDelay = time.Second * 3
	confPerm    = os.FileMode(0664) // rw-rw-r
)

var (
	globalOptions = []string{
		//		"chroot /var/lib/haproxy",
		"daemon",
		//		"user haproxy",
		//		"group haproxy",
		"log /dev/log local0",
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
	go s.configLoop()
	s.listenSignals()
}

// configLoop updates the haproxy config, and then waits
// for changes in the backend.
func (s *Service) configLoop() {
	for {
		if err := s.updateHaproxy(); err != nil {
			s.Logger.Error("Failed to update haproxy: %#v", err)
		}
		if err := s.Backend.Watch(); err != nil {
			s.Logger.Error("Failed to watch for backend changes: %#v", err)
		}
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
		return nil
	}

	// Validate the config
	if err := s.validateConfig(tempConf); err != nil {
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

// createConfig creates a new haproxy configuration file.
// It returns the path of the new config file.
func (s *Service) createConfig() (string, string, error) {
	c := haproxy.NewConfig()
	c.Section("global").Add(globalOptions...)
	c.Section("defaults").Add(defaultsOptions...)

	// Fetch data from backend
	frontends, err := s.Backend.FrontEnds()
	if err != nil {
		return "", "", maskAny(err)
	}
	services, err := s.Backend.Services()
	if err != nil {
		return "", "", maskAny(err)
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

	// Create config for all registrations
	frontEndSection := c.Section("frontend http-in")
	frontEndSection.Add("bind *:80")
	// Collect certificates
	certs := []string{}
	for _, fr := range frontends {
		for _, sel := range fr.Selectors {
			if sel.SslCert != "" {
				crt := fmt.Sprintf("crt %s", filepath.Join(s.SslCertsFolder, sel.SslCert))
				certs = append(certs, crt)
			}
		}
	}
	if len(certs) > 0 {
		frontEndSection.Add(
			fmt.Sprintf("bind *:443 ssl %s no-sslv3", strings.Join(certs, " ")),
		)
	}
	if s.ForceSsl {
		frontEndSection.Add("redirect scheme https if !{ ssl_fc }")
	}
	frontEndSection.Add(
		"reqadd X-Forwarded-Port:\\ %[dst_port]",
		"reqadd X-Forwarded-Proto:\\ https if { ssl_fc }",
		"default_backend fallback",
	)
	for _, fr := range frontends {
		// Create acls
		for _, sel := range fr.Selectors {
			if sel.Domain != "" {
				if sel.SslCert != "" {
					frontEndSection.Add(fmt.Sprintf("acl %s ssl_fc_sni -i %s", fr.aclName(), sel.Domain))
				} else {
					frontEndSection.Add(fmt.Sprintf("acl %s hdr_dom(host) -i %s", fr.aclName(), sel.Domain))
				}
			}
			if sel.PathPrefix != "" {
				frontEndSection.Add(fmt.Sprintf("acl %s path_beg %s", fr.aclName(), sel.PathPrefix))
			}
		}

		// Create link to backend
		frontEndSection.Add(fmt.Sprintf("use_backend %s if %s", fr.backendName(), fr.aclName()))

		// Create backend
		backendSection := c.Section(fmt.Sprintf("backend %s", fr.backendName()))
		backendSection.Add(
			"mode http",
			"balance roundrobin",
		)
		if fr.HttpCheckPath != "" {
			backendSection.Add(fmt.Sprintf("option httpchk GET %s", fr.HttpCheckPath))
		}
		for _, service := range services {
			if service.Name != fr.Service {
				continue
			}
			for i, b := range service.Backends {
				id := fmt.Sprintf("%s-%d", service.Name, i)
				check := ""
				if fr.HttpCheckPath != "" {
					check = "check"
				}
				backendSection.Add(fmt.Sprintf("server %s %s %s", id, b, check))
			}
		}
	}

	// Create fallback backend
	fbbSection := c.Section("backend fallback")
	fbbSection.Add(
		"mode http",
		"balance roundrobin",
	)

	// Render config
	config := c.Render()
	s.Logger.Debug("Config:\n%s", config)

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

// validateConfig calls haproxy to validate the given config file.
func (s *Service) validateConfig(confPath string) error {
	cmd := exec.Command(s.HaproxyPath, "-c", "-f", confPath)
	output, err := cmd.Output()
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
		"-p",
		s.HaproxyPidPath,
	}
	if pid, err := ioutil.ReadFile(s.HaproxyPidPath); err == nil {
		args = append(args, "-st", string(pid))
	}

	s.Logger.Debug("Starting haproxy with %#v", args)
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("%s %s", s.HaproxyPath, strings.Join(args, " ")))
	if err := cmd.Start(); err != nil {
		s.Logger.Error("Failed to start haproxy: %#v", err)
		return maskAny(err)
	}
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
