package acme

import (
	"crypto/rsa"

	"github.com/xenolf/lego/acme"
)

type acmeUser struct {
	Email        string
	Registration *acme.RegistrationResource
	PrivateKey   *rsa.PrivateKey
}

func (u acmeUser) GetEmail() string {
	return u.Email
}

func (u acmeUser) GetRegistration() *acme.RegistrationResource {
	return u.Registration
}

func (u acmeUser) GetPrivateKey() *rsa.PrivateKey {
	return u.PrivateKey
}
