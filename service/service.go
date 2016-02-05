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
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/op/go-logging"

	"git.pulcy.com/pulcy/load-balancer/service/acme"
	"git.pulcy.com/pulcy/load-balancer/service/backend"
)

const (
	osExitDelay  = time.Second * 3
	confPerm     = os.FileMode(0664) // rw-rw-r
	refreshDelay = time.Second * 5
)

type ServiceConfig struct {
	HaproxyConfPath   string
	HaproxyPath       string
	HaproxyPidPath    string
	StatsPort         int
	StatsUser         string
	StatsPassword     string
	StatsSslCert      string
	SslCertsFolder    string
	ForceSsl          bool
	PrivateHost       string
	PrivateTcpSslCert string // Name of SSL certificate used for private tcp connections
}

type ServiceDependencies struct {
	Logger      *logging.Logger
	Backend     backend.Backend
	AcmeService acme.AcmeService
}

type Service struct {
	ServiceConfig
	ServiceDependencies

	signalCounter uint32
	lastConfig    string
	lastPid       int
	changeCounter uint32
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
		s.TriggerUpdate()
	}()
	s.listenSignals()
}

// configLoop updates the haproxy config, and then waits
// for changes in the backend.
func (s *Service) configLoop() {
	var lastChangeCounter uint32
	for {
		currentChangeCounter := atomic.LoadUint32(&s.changeCounter)
		if currentChangeCounter > lastChangeCounter {
			if err := s.updateHaproxy(); err != nil {
				s.Logger.Error("Failed to update haproxy: %#v", err)
			} else {
				// Success
				lastChangeCounter = currentChangeCounter
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
		s.TriggerUpdate()
	}
}

// TriggerUpdate notifies the service to update the haproxy configuration
func (s *Service) TriggerUpdate() {
	atomic.AddUint32(&s.changeCounter, 1)
}

// update the haproxy configuration
func (s *Service) updateHaproxy() error {
	// Create a new config (in temp path)
	config, tempConf, err := s.createConfigFile()
	if err != nil {
		return maskAny(err)
	}

	// If nothing has changed, no temp file is created, then do nothing
	if tempConf == "" {
		return nil
	}

	// Cleanup afterwards
	defer os.Remove(tempConf)

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

// createConfigFile creates a new haproxy configuration file.
// It returns the path of the new config file.
func (s *Service) createConfigFile() (string, string, error) {
	// Fetch data from backend
	services, err := s.Backend.Services()
	if err != nil {
		return "", "", maskAny(err)
	}

	// Extend with ACME info
	services, err = s.AcmeService.Extend(services)
	if err != nil {
		return "", "", maskAny(err)
	}

	// Sort the services
	services.Sort()

	// Render the content of the haproxy.cfg file
	config, err := s.renderConfig(services)
	if err != nil {
		return "", "", maskAny(err)
	}

	// If nothing has changed, don't do anything
	if s.lastConfig == config {
		s.Logger.Debug("Config has not changed")
		return config, "", nil
	}

	// Log services
	s.Logger.Info("Found %d services", len(services))
	for srvIndex, srv := range services {
		s.Logger.Debug("Service %d: %#v", srvIndex, srv)
	}

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
	configureRestartHaproxyCmd(cmd)
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
