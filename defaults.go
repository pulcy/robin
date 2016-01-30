package main

import (
	"github.com/mitchellh/go-homedir"
)

const (
	defaultStatsPort      = 7088
	defaultStatsUser      = ""
	defaultStatsPassword  = ""
	defaultStatsSslCert   = ""
	defaultSslCertsFolder = "/certs/"
	defaultForceSsl       = false
	defaultPrivateHost    = ""
	defaultAcmeHttpPort   = 8011
)

const (
	defaultKeyBits              = 2048
	defaultCADirectoryURL       = "https://acme-v01.api.letsencrypt.org/directory"
	defaultPrivateKeyPathTmpl   = "~/.pulcy/acme/private-key.pem"
	defaultRegistrationPathTmpl = "~/.pulcy/acme/registration.json"
)

func defaultPrivateKeyPath() string {
	result, err := homedir.Expand(defaultPrivateKeyPathTmpl)
	if err != nil {
		Exitf("Cannot expand private-key-path: %#v", err)
	}
	return result
}

func defaultRegistrationPath() string {
	result, err := homedir.Expand(defaultRegistrationPathTmpl)
	if err != nil {
		Exitf("Cannot expand registration-path: %#v", err)
	}
	return result
}
