package acme

import (
	"crypto/rsa"
	"fmt"

	"github.com/xenolf/lego/acme"

	"git.pulcy.com/pulcy/load-balancer/service/backend"
	"git.pulcy.com/pulcy/load-balancer/service/locks"
)

const (
	acmeServiceName = "__acme"
	acmeServicePort = 0
)

type AcmeServiceConfig struct {
	HttpProviderConfig

	EtcdPrefix       string // Folder in ETCD to use ACME
	CADirectoryURL   string // URL of ACME directory
	KeyBits          int    // Size of generated keys (in bits)
	Email            string // Registration email address
	PrivateKeyPath   string // Path of file containing private key
	RegistrationPath string // Path of file containing acme.RegistrationResource
}

type AcmeServiceDependencies struct {
	HttpProviderDependencies

	LockService locks.LockService
}

type AcmeService interface {
	Register() error
	Start() error
	Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error)
}

type acmeService struct {
	AcmeServiceConfig
	AcmeServiceDependencies

	httpProvider                *httpChallengeProvider
	acmeClient                  *acme.Client
	privateKey                  *rsa.PrivateKey
	domainCertificatesWaitIndex uint64
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
	// Check arguments
	missingArgs := []string{}
	if s.Email == "" {
		missingArgs = append(missingArgs, "acme-email")
	}
	if s.CADirectoryURL == "" {
		missingArgs = append(missingArgs, "acme-directory-url")
	}
	if s.PrivateKeyPath == "" {
		missingArgs = append(missingArgs, "private-key-path")
	}
	if s.RegistrationPath == "" {
		missingArgs = append(missingArgs, "registration-path")
	}

	if len(missingArgs) > 0 {
		s.Logger.Warning("ACME is not configured, some it will not be used. Missing: %v", missingArgs)
		return nil
	}

	// Load private key
	key, err := s.getPrivateKey()
	if err != nil {
		return maskAny(err)
	}

	// Load registration
	registration, err := s.getRegistration()
	if err != nil {
		return maskAny(err)
	}
	if registration == nil {
		return maskAny(fmt.Errorf("No registration found at %s", s.RegistrationPath))
	}

	// Create ACME client
	user := acmeUser{
		Email:        s.Email,
		Registration: registration,
		PrivateKey:   key,
	}
	client, err := acme.NewClient(s.CADirectoryURL, user, s.KeyBits)
	if err != nil {
		return maskAny(err)
	}

	// Save objects
	s.privateKey = key
	s.acmeClient = client

	// Start HTTP challenge listener
	if err := s.httpProvider.Start(); err != nil {
		return maskAny(err)
	}
	return nil
}

// Extend fills is missing data provided by ACME into the list of services.
// It also adds a service to handle ACME HTTP challenges
func (s *acmeService) Extend(services backend.ServiceRegistrations) (backend.ServiceRegistrations, error) {
	// Find domains that need a certificate
	domainSet := make(map[string]struct{})
	domains := []string{}
	for _, sr := range services {
		for _, sel := range sr.Selectors {
			if sel.Private || sel.SslCert != "" || sel.Domain == "" {
				continue
			}
			if _, ok := domainSet[sel.Domain]; !ok {
				domainSet[sel.Domain] = struct{}{}
				domains = append(domains, sel.Domain)
			}
		}
	}

	// Request certificates for the domains
	if len(domains) > 0 {
		go func() {
			// Now request the certificates
			if err := s.requestCertificates(domains); err != nil {
				s.Logger.Error("Failed to request certificates: %#v", err)
			}
		}()
	}

	// Add HTTP challenge service
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
