package acme

import (
	"bufio"
	"fmt"
	"os"

	"github.com/xenolf/lego/acme"
)

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

		user.Registration = registration
		client, err = acme.NewClient(s.CADirectoryURL, user, s.KeyBits)
		if err != nil {
			return maskAny(err)
		}
	}

	fmt.Printf("Find the terms here:%s\n", registration.TosURL)
	if err := confirm("Do you agree with these terms?"); err != nil {
		return maskAny(err)
	}

	if err := client.AgreeToTOS(); err != nil {
		return maskAny(err)
	}

	fmt.Printf(`
Registration succeeded:

Email       : %s
Private key : %s
Registration: %s

Save these files in a secure location.
`, s.Email, s.PrivateKeyPath, s.RegistrationPath)

	return nil
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
