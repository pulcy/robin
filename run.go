package main

import (
	"net/url"

	"github.com/coreos/go-etcd/etcd"
	"github.com/spf13/cobra"

	"git.pulcy.com/pulcy/load-balancer/service"
	"git.pulcy.com/pulcy/load-balancer/service/acme"
	"git.pulcy.com/pulcy/load-balancer/service/backend"
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

var (
	cmdRun = &cobra.Command{
		Use:   "run",
		Short: "Run the load-balancer",
		Long:  "Run the load-balancer",
		Run:   cmdRunRun,
	}

	runArgs struct {
		backendEtcdAddr string
		acmeEtcdAddr    string
		acmeHttpPort    int
		haproxyConfPath string
		statsPort       int
		statsUser       string
		statsPassword   string
		statsSslCert    string
		sslCertsFolder  string
		forceSsl        bool
		privateHost     string
	}
)

func init() {
	cmdRun.Flags().StringVar(&runArgs.backendEtcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdRun.Flags().StringVar(&runArgs.acmeEtcdAddr, "acme-etcd-addr", "", "Address of etcd acme")
	cmdRun.Flags().StringVar(&runArgs.haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
	cmdRun.Flags().IntVar(&runArgs.statsPort, "stats-port", defaultStatsPort, "Port for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsUser, "stats-user", defaultStatsUser, "User for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsPassword, "stats-password", defaultStatsPassword, "Password for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsSslCert, "stats-ssl-cert", defaultStatsSslCert, "Filename of SSL certificate for stats page (located in ss-certs)")
	cmdRun.Flags().StringVar(&runArgs.sslCertsFolder, "ssl-certs", defaultSslCertsFolder, "Folder containing SSL certificate")
	cmdRun.Flags().BoolVar(&runArgs.forceSsl, "force-ssl", defaultForceSsl, "Redirect HTTP to HTTPS")
	cmdRun.Flags().StringVar(&runArgs.privateHost, "private-host", defaultPrivateHost, "IP address of private network")
	cmdRun.Flags().IntVar(&runArgs.acmeHttpPort, "acme-http-port", defaultAcmeHttpPort, "Port to listen for ACME HTTP challenges on (internally)")
	cmdMain.AddCommand(cmdRun)
}

func cmdRunRun(cmd *cobra.Command, args []string) {
	// Prepare backend
	if runArgs.backendEtcdAddr == "" {
		Exitf("Please specify --etcd-addr")
	}
	backendEtcdUrl, err := url.Parse(runArgs.backendEtcdAddr)
	if err != nil {
		Exitf("--etcd-addr '%s' is not valid: %#v", runArgs.backendEtcdAddr, err)
	}
	backend := backend.NewEtcdBackend(log, backendEtcdUrl)

	// Prepare acme service
	if runArgs.acmeEtcdAddr == "" {
		Exitf("Please specify --acme-etcd-addr")
	}
	acmeEtcdUrl, err := url.Parse(runArgs.acmeEtcdAddr)
	if err != nil {
		Exitf("--acme-etcd-addr '%s' is not valid: %#v", runArgs.acmeEtcdAddr, err)
	}
	acmeEtcdClient := etcd.NewClient([]string{"http://" + acmeEtcdUrl.Host})
	acmeService := acme.NewAcmeService(acme.AcmeServiceConfig{
		HttpProviderConfig: acme.HttpProviderConfig{
			EtcdPrefix: acmeEtcdUrl.Path,
			Port:       runArgs.acmeHttpPort,
		},
	}, acme.AcmeServiceDependencies{
		HttpProviderDependencies: acme.HttpProviderDependencies{
			Logger:     log,
			EtcdClient: acmeEtcdClient,
		},
	})

	// Prepare service
	if runArgs.haproxyConfPath == "" {
		Exitf("Please specify --haproxy-conf")
	}
	if runArgs.privateHost == "" {
		Exitf("Please specify --private-host")
	}
	service := service.NewService(service.ServiceConfig{
		HaproxyConfPath: runArgs.haproxyConfPath,
		StatsPort:       runArgs.statsPort,
		StatsUser:       runArgs.statsUser,
		StatsPassword:   runArgs.statsPassword,
		StatsSslCert:    runArgs.statsSslCert,
		SslCertsFolder:  runArgs.sslCertsFolder,
		ForceSsl:        runArgs.forceSsl,
		PrivateHost:     runArgs.privateHost,
	}, service.ServiceDependencies{
		Logger:      log,
		Backend:     backend,
		AcmeService: acmeService,
	})

	// Start all services
	if err := acmeService.Start(); err != nil {
		Exitf("Failed to start ACME service: %#v", err)
	}
	service.Run()
}
