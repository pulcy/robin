package acme

import (
	"github.com/xenolf/lego/acme"

	"git.pulcy.com/pulcy/load-balancer/service/backend"
)

const (
	acmeServiceName = "__acme"
	acmeServicePort = 0
)

type AcmeServiceConfig struct {
	HttpProviderConfig
}

type AcmeServiceDependencies struct {
	HttpProviderDependencies
}

type AcmeService interface {
	Start() error
	Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error)
}

type acmeService struct {
	AcmeServiceConfig
	AcmeServiceDependencies

	httpProvider *httpChallengeProvider
}

// NewAcmeService creates and initializes a new AcmeService implementation.
func NewAcmeService(config AcmeServiceConfig, deps AcmeServiceDependencies) AcmeService {
	return &acmeService{
		AcmeServiceConfig:       config,
		AcmeServiceDependencies: deps,

		httpProvider: newHttpChallengeProvider(config.HttpProviderConfig, deps.HttpProviderDependencies),
	}
}

// Start launches this services.
func (s *acmeService) Start() error {
	if err := s.httpProvider.Start(); err != nil {
		return maskAny(err)
	}
	return nil
}

// Extend fills is missing data provided by ACME into the list of services.
// It also adds a service to handle ACME HTTP challenges
func (s *acmeService) Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error) {
	services = append(services, s.createAcmeServiceRegistration())
	return services, nil
}

func (s *acmeService) createAcmeServiceRegistration() backend.ServiceRegistration {
	pathPrefix := acme.HTTP01ChallengePath("")
	sr := backend.ServiceRegistration{
		ServiceName: acmeServiceName,
		ServicePort: acmeServicePort,
		Instances: backend.ServiceInstances{
			backend.ServiceInstance{
				IP:   "127.0.0.1",
				Port: s.HttpProviderConfig.Port,
			},
		},
		Selectors: backend.ServiceSelectors{
			backend.ServiceSelector{
				Weight:     100,
				PathPrefix: pathPrefix,
				Private:    false,
			},
		},
		HttpCheckPath: "",
	}
	return sr
}
