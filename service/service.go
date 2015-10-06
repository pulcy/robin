package service

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/op/go-logging"
)

const (
	osExitDelay = time.Second * 3
)

type ServiceConfig struct {
}

type ServiceDependencies struct {
	Logger *logging.Logger
}

type Service struct {
	ServiceConfig
	ServiceDependencies

	signalCounter uint32
}

func NewService(config ServiceConfig, deps ServiceDependencies) *Service {
	return &Service{
		ServiceConfig:       config,
		ServiceDependencies: deps,
	}
}

func (s *Service) Run() {
	go s.configLoop()
	s.listenSignals()
}

func (s *Service) configLoop() {

}

func (s *Service) close() {
	// Interrupt the process when closing is requested twice.
	if atomic.AddUint32(&s.signalCounter, 1) >= 2 {
		s.exitProcess()
	}

	s.Logger.Info("shutting down server in %s", osExitDelay.String())
	time.Sleep(osExitDelay)

	s.exitProcess()
}

func (s *Service) exitProcess() {
	s.Logger.Info("shutting down server")
	os.Exit(0)
}

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
