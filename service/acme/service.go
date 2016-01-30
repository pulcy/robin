package acme

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

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

// Register the account with the ACME server
func (s *acmeService) Register() error {
	key, err := s.getPrivateKey()
	if err != nil {
		return maskAny(err)
	}

	registration, err := s.getRegistration()
	if err != nil {
		return maskAny(err)
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

	if registration == nil {
		registration, err = client.Register()
		if err != nil {
			return maskAny(err)
		}
		if err := s.saveRegistration(registration); err != nil {
			return maskAny(err)
		}
	}

	resp, err := http.Get(registration.TosURL)
	if err != nil {
		return maskAny(err)
	}
	defer resp.Body.Close()
	tos, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return maskAny(err)
	}
	fmt.Println(string(tos))
	if err := confirm("Do you agree with these terms?"); err != nil {
		return maskAny(err)
	}

	if err := client.AgreeToTOS(); err != nil {
		return maskAny(err)
	}

	return nil
}

// Start launches this services.
func (s *acmeService) Start() error {
	var user acme.User // TODO
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

func confirm(question string) error {
	for {
		fmt.Printf("%s [yes|no]", question)
		bufStdin := bufio.NewReader(os.Stdin)
		line, _, err := bufStdin.ReadLine()
		if err != nil {
			return err
		}

		if string(line) == "yes" || string(line) == "y" {
			return nil
		}
		fmt.Println("Please enter 'yes' to confirm.")
	}
}
