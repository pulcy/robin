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

package main

import (
	"github.com/spf13/cobra"

	"git.pulcy.com/pulcy/load-balancer/service/acme"
)

var (
	cmdRegisterAcme = &cobra.Command{
		Use:   "acme",
		Short: "Register an account at an ACME server",
		Long:  "Register an account at an ACME server",
		Run:   cmdRegisterAcmeRun,
	}

	registerAcmeArgs struct {
		acmeEmail        string
		caDirURL         string
		keyBits          int
		privateKeyPath   string
		registrationPath string
	}
)

func init() {
	cmdRegisterAcme.Flags().StringVar(&registerAcmeArgs.acmeEmail, "acme-email", "", "Email account for ACME server")
	cmdRegisterAcme.Flags().StringVar(&registerAcmeArgs.caDirURL, "acme-directory-url", defaultCADirectoryURL, "Directory URL of the ACME server")
	cmdRegisterAcme.Flags().IntVar(&registerAcmeArgs.keyBits, "key-bits", defaultKeyBits, "Length of generated keys in bits")
	cmdRegisterAcme.Flags().StringVar(&registerAcmeArgs.privateKeyPath, "private-key-path", defaultPrivateKeyPath(), "Path of the private key for the registered account")
	cmdRegisterAcme.Flags().StringVar(&registerAcmeArgs.registrationPath, "registration-path", defaultRegistrationPath(), "Path of the registration resource for the registered account")
	cmdRegister.AddCommand(cmdRegisterAcme)
}

func cmdRegisterAcmeRun(cmd *cobra.Command, args []string) {
	if registerAcmeArgs.acmeEmail == "" {
		Exitf("Please specify --acme-email")
	}
	acmeService := acme.NewAcmeService(acme.AcmeServiceConfig{
		HttpProviderConfig: acme.HttpProviderConfig{},
		CADirectoryURL:     registerAcmeArgs.caDirURL,
		KeyBits:            registerAcmeArgs.keyBits,
		Email:              registerAcmeArgs.acmeEmail,
		PrivateKeyPath:     registerAcmeArgs.privateKeyPath,
		RegistrationPath:   registerAcmeArgs.registrationPath,
	}, acme.AcmeServiceDependencies{
		HttpProviderDependencies: acme.HttpProviderDependencies{
			Logger: log,
		},
	})

	// Perform registration
	if err := acmeService.Register(); err != nil {
		Exitf("Registration failed: %#v", err)
	}
}
