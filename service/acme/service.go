package acme

import (
	"fmt"

	"github.com/xenolf/lego/acme"

	"git.pulcy.com/pulcy/load-balancer/service/backend"
)

const (
	acmeServiceName = "__acme"
	acmeServicePort = 0
)

type AcmeServiceConfig struct {
	HttpProviderConfig

	CADirectoryURL   string // URL of ACME directory
	KeyBits          int    // Size of generated keys (in bits)
	Email            string // Registration email address
	PrivateKeyPath   string // Path of file containing private key
	RegistrationPath string // Path of file containing acme.RegistrationResource
}

type AcmeServiceDependencies struct {
	HttpProviderDependencies
}

type AcmeService interface {
	Register() error
	Start() error
	Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error)
}

type acmeService struct {
	AcmeServiceConfig
	AcmeServiceDependencies

	httpProvider *httpChallengeProvider
	acmeClient   *acme.Client
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
	if s.Email == "" {
		return maskAny(fmt.Errorf("Email empty"))
	}
	if s.CADirectoryURL == "" {
		return maskAny(fmt.Errorf("CADirectoryURL empty"))
	}
	if s.PrivateKeyPath == "" {
		return maskAny(fmt.Errorf("PrivateKeyPath empty"))
	}
	if s.RegistrationPath == "" {
		return maskAny(fmt.Errorf("RegistrationPath empty"))
	}
	key, err := s.getPrivateKey()
	if err != nil {
		return maskAny(err)
	}

	registration, err := s.getRegistration()
	if err != nil {
		return maskAny(err)
	}
	if registration == nil {
		return maskAny(fmt.Errorf("No registration found at %s", s.RegistrationPath))
	}

	user := acmeUser{
		Email:        s.Email,
		Registration: registration,
		PrivateKey:   key,
	}

	client, err := acme.NewClient(s.CADirectoryURL, user, s.KeyBits)
	if err != nil {
		return maskAny(err)
	}
	s.acmeClient = client

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
