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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"

	"github.com/pulcy/robin/metrics"
	"github.com/pulcy/robin/middleware"
	"github.com/pulcy/robin/service"
	"github.com/pulcy/robin/service/acme"
	"github.com/pulcy/robin/service/backend"
	"github.com/pulcy/robin/service/mutex"
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
		backend           string
		logLevel          string
		etcdAddr          string
		etcdEndpoints     []string
		etcdPath          string
		haproxyConfPath   string
		statsPort         int
		statsUser         string
		statsPassword     string
		statsSslCert      string
		sslCertsFolder    string
		forceSsl          bool
		privateHost       string
		publicHost        string
		privateTcpSslCert string
		excludePublic     bool
		excludePrivate    bool

		// acme
		acmeHttpPort       int
		acmeEmail          string
		caDirURL           string
		keyBits            int
		privateKeyPath     string
		registrationPath   string
		tmpCertificatePath string

		// metrics
		metricsHost      string
		metricsPort      int
		privateStatsPort int

		// api
		apiHost string
		apiPort int
	}
)

type acmeServiceListener struct {
	service *service.Service
}

func init() {
	defaultAcmeEmail := os.Getenv("ACME_EMAIL")
	defaultStatsPassword := os.Getenv("STATS_PASSWORD")
	defaultStatsUser := os.Getenv("STATS_USER")
	cmdRun.Flags().StringVar(&runArgs.backend, "backend", defaultBackend, "Used backend (etcd|kubernetes)")
	cmdRun.Flags().StringVar(&runArgs.logLevel, "log-level", defaultLogLevel, "Log level (debug|info|warning|error)")
	cmdRun.Flags().StringVar(&runArgs.etcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdRun.Flags().StringSliceVar(&runArgs.etcdEndpoints, "etcd-endpoint", nil, "Etcd client endpoints")
	cmdRun.Flags().StringVar(&runArgs.etcdPath, "etcd-path", "", "Path into etcd namespace")
	cmdRun.Flags().StringVar(&runArgs.haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
	cmdRun.Flags().IntVar(&runArgs.statsPort, "stats-port", defaultStatsPort, "Port for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsUser, "stats-user", defaultStatsUser, "User for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsPassword, "stats-password", defaultStatsPassword, "Password for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsSslCert, "stats-ssl-cert", defaultStatsSslCert, "Filename of SSL certificate for stats page (located in ssl-certs)")
	cmdRun.Flags().StringVar(&runArgs.sslCertsFolder, "ssl-certs", defaultSslCertsFolder, "Folder containing SSL certificate")
	cmdRun.Flags().BoolVar(&runArgs.forceSsl, "force-ssl", defaultForceSsl, "Redirect HTTP to HTTPS")
	cmdRun.Flags().StringVar(&runArgs.privateHost, "private-host", defaultPrivateHost, "IP address of private network")
	cmdRun.Flags().StringVar(&runArgs.publicHost, "public-host", defaultPublicHost, "IP address of public network")
	cmdRun.Flags().StringVar(&runArgs.privateTcpSslCert, "private-ssl-cert", defaultPrivateTcpSslCert, "Filename of SSL certificate for private TCP connections (located in ssl-certs)")
	cmdRun.Flags().BoolVar(&runArgs.excludePrivate, "exclude-private", false, "Exclude private frontends")
	cmdRun.Flags().BoolVar(&runArgs.excludePublic, "exclude-public", false, "Exclude public frontends")

	// acme
	cmdRun.Flags().IntVar(&runArgs.acmeHttpPort, "acme-http-port", defaultAcmeHttpPort, "Port to listen for ACME HTTP challenges on (internally)")
	cmdRun.Flags().StringVar(&runArgs.acmeEmail, "acme-email", defaultAcmeEmail, "Email account for ACME server")
	cmdRun.Flags().StringVar(&runArgs.caDirURL, "acme-directory-url", defaultCADirectoryURL, "Directory URL of the ACME server")
	cmdRun.Flags().IntVar(&runArgs.keyBits, "key-bits", defaultKeyBits, "Length of generated keys in bits")
	cmdRun.Flags().StringVar(&runArgs.privateKeyPath, "private-key-path", defaultPrivateKeyPath(), "Path of the private key for the registered account")
	cmdRun.Flags().StringVar(&runArgs.registrationPath, "registration-path", defaultRegistrationPath(), "Path of the registration resource for the registered account")
	cmdRun.Flags().StringVar(&runArgs.tmpCertificatePath, "tmp-certificate-path", defaultTmpCertificatePath, "Path of obtained tmp certificates")

	// metrics
	cmdRun.Flags().StringVar(&runArgs.metricsHost, "metrics-host", defaultMetricsHost, "Host address to listen for metrics requests")
	cmdRun.Flags().IntVar(&runArgs.metricsPort, "metrics-port", defaultMetricsPort, "Port to listen for metrics requests")
	cmdRun.Flags().IntVar(&runArgs.privateStatsPort, "private-stats-port", defaultPrivateStatsPort, "HAProxy port CSV stats")

	// api
	cmdRun.Flags().StringVar(&runArgs.apiHost, "api-host", defaultApiHost, "Host address to listen for API requests")
	cmdRun.Flags().IntVar(&runArgs.apiPort, "api-port", defaultApiPort, "Port to listen for API requests")

	cmdMain.AddCommand(cmdRun)
}

func cmdRunRun(cmd *cobra.Command, args []string) {
	// Parse arguments
	if runArgs.etcdAddr != "" {
		etcdUrl, err := url.Parse(runArgs.etcdAddr)
		if err != nil {
			Exitf("--etcd-addr '%s' is not valid: %#v", runArgs.etcdAddr, err)
		}
		runArgs.etcdEndpoints = []string{fmt.Sprintf("%s://%s", etcdUrl.Scheme, etcdUrl.Host)}
		runArgs.etcdPath = etcdUrl.Path
	}
	etcdCfg := client.Config{
		Endpoints: runArgs.etcdEndpoints,
		Transport: client.DefaultTransport,
	}
	etcdClient, err := client.New(etcdCfg)
	if err != nil {
		Exitf("Failed to initialize ETCD client: %#v", err)
	}

	go etcdClient.AutoSync(context.Background(), time.Second*30)

	// Set log level
	level, err := logging.LogLevel(runArgs.logLevel)
	if err != nil {
		Exitf("Invalid log-level '%s': %#v", runArgs.logLevel, err)
	}
	logging.SetLevel(level, cmdMain.Use)

	// Prepare backend
	var b backend.Backend
	switch runArgs.backend {
	case "etcd":
		b, err = backend.NewEtcdBackend(etcdBackendConfig, log, etcdClient, runArgs.etcdPath)
		if err != nil {
			Exitf("Failed to create ETCD backend: %#v", err)
		}
	case "kubernetes":
		b, err = backend.NewKubernetesBackend(etcdBackendConfig, log)
		if err != nil {
			Exitf("Failed to create Kubernetes backend: %#v", err)
		}
	default:
		Exitf("Unknown backend: '%s'", runArgs.backend)
	}

	// Prepare global mutext service
	gmService := mutex.NewEtcdGlobalMutexService(etcdClient, path.Join(runArgs.etcdPath, etcdLocksFolder))

	// Prepare acme service
	acmeEtcdPrefix := path.Join(runArgs.etcdPath, etcdAcmeFolder)
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
		PrivateStatsPort:  runArgs.privateStatsPort,
		ExcludePrivate:    runArgs.excludePrivate,
		ExcludePublic:     runArgs.excludePublic,
	}, service.ServiceDependencies{
		Logger:      log,
		Backend:     b,
		AcmeService: acmeService,
	})
	acmeServiceListener.service = service

	// Prepare and run middleware
	apiMiddleware := middleware.Middleware{
		Logger:  log,
		Service: b,
	}
	apiAddr := fmt.Sprintf("%s:%d", runArgs.apiHost, runArgs.apiPort)
	apiHandler := apiMiddleware.SetupRoutes(projectName, projectVersion, projectBuild)
	log.Infof("Starting %s API (version %s build %s) on %s\n", projectName, projectVersion, projectBuild, apiAddr)
	go func() {
		if err := http.ListenAndServe(apiAddr, apiHandler); err != nil {
			log.Fatalf("API ListenAndServe failed: %#v", err)
		}
	}()

	// Start all services
	if err := acmeService.Start(); err != nil {
		Exitf("Failed to start ACME service: %#v", err)
	}
	metricsConfig := metrics.MetricsConfig{
		ProjectName:    projectName,
		ProjectVersion: projectVersion,
		ProjectBuild:   projectBuild,
		Host:           runArgs.metricsHost,
		Port:           runArgs.metricsPort,
		HaproxyCSVURI:  fmt.Sprintf("http://127.0.0.1:%d/;csv", runArgs.privateStatsPort),
	}
	if runArgs.privateStatsPort == 0 {
		metricsConfig.HaproxyCSVURI = ""
	}
	if err := metrics.StartMetricsListener(metricsConfig, log); err != nil {
		Exitf("Failed to start metrics: %#v", err)
	}
	service.Run()
}

// CertificatesUpdated is called when there is a change in one of the ACME generated certificates
func (l *acmeServiceListener) CertificatesUpdated() {
	if l.service != nil {
		l.service.TriggerUpdate()
	}
}
