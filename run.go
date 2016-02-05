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
	"net/url"
	"os"
	"path"

	"github.com/coreos/etcd/client"
	"github.com/spf13/cobra"

	"git.pulcy.com/pulcy/load-balancer/service"
	"git.pulcy.com/pulcy/load-balancer/service/acme"
	"git.pulcy.com/pulcy/load-balancer/service/backend"
	"git.pulcy.com/pulcy/load-balancer/service/mutex"
)

const (
	etcdLocksFolder = "lb/locks"
	etcdAcmeFolder  = "lb/acme"
)

var (
	cmdRun = &cobra.Command{
		Use:   "run",
		Short: "Run the load-balancer",
		Long:  "Run the load-balancer",
		Run:   cmdRunRun,
	}

	runArgs struct {
		etcdAddr          string
		haproxyConfPath   string
		statsPort         int
		statsUser         string
		statsPassword     string
		statsSslCert      string
		sslCertsFolder    string
		forceSsl          bool
		privateHost       string
		privateTcpSslCert string

		// acme
		acmeHttpPort       int
		acmeEmail          string
		caDirURL           string
		keyBits            int
		privateKeyPath     string
		registrationPath   string
		tmpCertificatePath string
	}
)

type acmeServiceListener struct {
	service *service.Service
}

func init() {
	defaultAcmeEmail := os.Getenv("ACME_EMAIL")
	defaultStatsPassword := os.Getenv("STATS_PASSWORD")
	defaultStatsUser := os.Getenv("STATS_USER")
	cmdRun.Flags().StringVar(&runArgs.etcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdRun.Flags().StringVar(&runArgs.haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
	cmdRun.Flags().IntVar(&runArgs.statsPort, "stats-port", defaultStatsPort, "Port for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsUser, "stats-user", defaultStatsUser, "User for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsPassword, "stats-password", defaultStatsPassword, "Password for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsSslCert, "stats-ssl-cert", defaultStatsSslCert, "Filename of SSL certificate for stats page (located in ssl-certs)")
	cmdRun.Flags().StringVar(&runArgs.sslCertsFolder, "ssl-certs", defaultSslCertsFolder, "Folder containing SSL certificate")
	cmdRun.Flags().BoolVar(&runArgs.forceSsl, "force-ssl", defaultForceSsl, "Redirect HTTP to HTTPS")
	cmdRun.Flags().StringVar(&runArgs.privateHost, "private-host", defaultPrivateHost, "IP address of private network")
	cmdRun.Flags().StringVar(&runArgs.privateTcpSslCert, "private-ssl-cert", defaultPrivateTcpSslCert, "Filename of SSL certificate for private TCP connections (located in ssl-certs)")
	// acme
	cmdRun.Flags().IntVar(&runArgs.acmeHttpPort, "acme-http-port", defaultAcmeHttpPort, "Port to listen for ACME HTTP challenges on (internally)")
	cmdRun.Flags().StringVar(&runArgs.acmeEmail, "acme-email", defaultAcmeEmail, "Email account for ACME server")
	cmdRun.Flags().StringVar(&runArgs.caDirURL, "acme-directory-url", defaultCADirectoryURL, "Directory URL of the ACME server")
	cmdRun.Flags().IntVar(&runArgs.keyBits, "key-bits", defaultKeyBits, "Length of generated keys in bits")
	cmdRun.Flags().StringVar(&runArgs.privateKeyPath, "private-key-path", defaultPrivateKeyPath(), "Path of the private key for the registered account")
	cmdRun.Flags().StringVar(&runArgs.registrationPath, "registration-path", defaultRegistrationPath(), "Path of the registration resource for the registered account")
	cmdRun.Flags().StringVar(&runArgs.tmpCertificatePath, "tmp-certificate-path", defaultTmpCertificatePath, "Path of obtained tmp certificates")

	cmdMain.AddCommand(cmdRun)
}

func cmdRunRun(cmd *cobra.Command, args []string) {
	// Parse arguments
	if runArgs.etcdAddr == "" {
		Exitf("Please specify --etcd-addr")
	}
	etcdUrl, err := url.Parse(runArgs.etcdAddr)
	if err != nil {
		Exitf("--etcd-addr '%s' is not valid: %#v", runArgs.etcdAddr, err)
	}
	etcdCfg := client.Config{
		Endpoints: []string{"http://" + etcdUrl.Host},
		Transport: client.DefaultTransport,
	}
	etcdClient, err := client.New(etcdCfg)
	if err != nil {
		Exitf("Failed to initialize ETCD client: %#v", err)
	}

	// Prepare backend
	backend, err := backend.NewEtcdBackend(log, etcdUrl)
	if err != nil {
		Exitf("Failed to backend: %#v", err)
	}

	// Prepare global mutext service
	gmService := mutex.NewEtcdGlobalMutexService(etcdClient, path.Join(etcdUrl.Path, etcdLocksFolder))

	// Prepare acme service
	acmeEtcdPrefix := path.Join(etcdUrl.Path, etcdAcmeFolder)
	certsRepository := acme.NewEtcdCertificatesRepository(acmeEtcdPrefix, etcdClient)
	certsCache := acme.NewCertificatesFileCache(runArgs.tmpCertificatePath, certsRepository, log)
	certsRequester := acme.NewCertificateRequester(log, certsRepository, gmService)
	renewal := acme.NewRenewalMonitor(log, certsRepository, certsRequester)
	acmeServiceListener := &acmeServiceListener{}
	acmeService := acme.NewAcmeService(acme.AcmeServiceConfig{
		HttpProviderConfig: acme.HttpProviderConfig{
			EtcdPrefix: acmeEtcdPrefix,
			Port:       runArgs.acmeHttpPort,
		},
		EtcdPrefix:       acmeEtcdPrefix,
		CADirectoryURL:   runArgs.caDirURL,
		KeyBits:          runArgs.keyBits,
		Email:            runArgs.acmeEmail,
		PrivateKeyPath:   runArgs.privateKeyPath,
		RegistrationPath: runArgs.registrationPath,
	}, acme.AcmeServiceDependencies{
		HttpProviderDependencies: acme.HttpProviderDependencies{
			Logger:     log,
			EtcdClient: etcdClient,
		},
		Listener:   acmeServiceListener,
		Repository: certsRepository,
		Cache:      certsCache,
		Renewal:    renewal,
		Requester:  certsRequester,
	})

	// Prepare service
	if runArgs.haproxyConfPath == "" {
		Exitf("Please specify --haproxy-conf")
	}
	if runArgs.privateHost == "" {
		Exitf("Please specify --private-host")
	}
	service := service.NewService(service.ServiceConfig{
		HaproxyConfPath:   runArgs.haproxyConfPath,
		StatsPort:         runArgs.statsPort,
		StatsUser:         runArgs.statsUser,
		StatsPassword:     runArgs.statsPassword,
		StatsSslCert:      runArgs.statsSslCert,
		SslCertsFolder:    runArgs.sslCertsFolder,
		ForceSsl:          runArgs.forceSsl,
		PrivateHost:       runArgs.privateHost,
		PrivateTcpSslCert: runArgs.privateTcpSslCert,
	}, service.ServiceDependencies{
		Logger:      log,
		Backend:     backend,
		AcmeService: acmeService,
	})
	acmeServiceListener.service = service

	// Start all services
	if err := acmeService.Start(); err != nil {
		Exitf("Failed to start ACME service: %#v", err)
	}
	service.Run()
}

// CertificatesUpdated is called when there is a change in one of the ACME generated certificates
func (l *acmeServiceListener) CertificatesUpdated() {
	if l.service != nil {
		l.service.TriggerUpdate()
	}
}
