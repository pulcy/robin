package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/op/go-logging"
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
	projectVersion = "dev"
	projectBuild   = "dev"
)

var (
	cmdMain = &cobra.Command{
		Use:   "load-balancer",
		Short: "Distribute incoming requests onto configurable backends",
		Long:  "Distribute incoming requests onto configurable backends",
	}
	log = logging.MustGetLogger(cmdMain.Use)

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
)

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:-5s}] %{message}"))
	cmdMain.Flags().StringVar(&backendEtcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdMain.Flags().StringVar(&acmeEtcdAddr, "acme-etcd-addr", "", "Address of etcd acme")
	cmdMain.Flags().StringVar(&haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
	cmdMain.Flags().IntVar(&statsPort, "stats-port", defaultStatsPort, "Port for stats page")
	cmdMain.Flags().StringVar(&statsUser, "stats-user", defaultStatsUser, "User for stats page")
	cmdMain.Flags().StringVar(&statsPassword, "stats-password", defaultStatsPassword, "Password for stats page")
	cmdMain.Flags().StringVar(&statsSslCert, "stats-ssl-cert", defaultStatsSslCert, "Filename of SSL certificate for stats page (located in ss-certs)")
	cmdMain.Flags().StringVar(&sslCertsFolder, "ssl-certs", defaultSslCertsFolder, "Folder containing SSL certificate")
	cmdMain.Flags().BoolVar(&forceSsl, "force-ssl", defaultForceSsl, "Redirect HTTP to HTTPS")
	cmdMain.Flags().StringVar(&privateHost, "private-host", defaultPrivateHost, "IP address of private network")
	cmdMain.Flags().IntVar(&acmeHttpPort, "acme-http-port", defaultAcmeHttpPort, "Port to listen for ACME HTTP challenges on (internally)")
	cmdMain.Run = cmdMainRun
}

func main() {
	cmdMain.Execute()
}

func cmdMainRun(cmd *cobra.Command, args []string) {
	// Prepare backend
	if backendEtcdAddr == "" {
		Exitf("Please specify --etcd-addr")
	}
	backendEtcdUrl, err := url.Parse(backendEtcdAddr)
	if err != nil {
		Exitf("--etcd-addr '%s' is not valid: %#v", backendEtcdAddr, err)
	}
	backend := backend.NewEtcdBackend(log, backendEtcdUrl)

	// Prepare acme service
	if acmeEtcdAddr == "" {
		Exitf("Please specify --acme-etcd-addr")
	}
	acmeEtcdUrl, err := url.Parse(acmeEtcdAddr)
	if err != nil {
		Exitf("--acme-etcd-addr '%s' is not valid: %#v", acmeEtcdAddr, err)
	}
	acmeEtcdClient := etcd.NewClient([]string{"http://" + acmeEtcdUrl.Host})
	acmeService := acme.NewAcmeService(acme.AcmeServiceConfig{
		HttpProviderConfig: acme.HttpProviderConfig{
			EtcdPrefix: acmeEtcdUrl.Path,
			Port:       acmeHttpPort,
		},
	}, acme.AcmeServiceDependencies{
		HttpProviderDependencies: acme.HttpProviderDependencies{
			Logger:     log,
			EtcdClient: acmeEtcdClient,
		},
	})

	// Prepare service
	if haproxyConfPath == "" {
		Exitf("Please specify --haproxy-conf")
	}
	if privateHost == "" {
		Exitf("Please specify --private-host")
	}
	service := service.NewService(service.ServiceConfig{
		HaproxyConfPath: haproxyConfPath,
		StatsPort:       statsPort,
		StatsUser:       statsUser,
		StatsPassword:   statsPassword,
		StatsSslCert:    statsSslCert,
		SslCertsFolder:  sslCertsFolder,
		ForceSsl:        forceSsl,
		PrivateHost:     privateHost,
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

func Exitf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format = format + "\n"
	}
	fmt.Printf(format, args...)
	os.Exit(1)
}

func def(envKey, defaultValue string) string {
	s := os.Getenv(envKey)
	if s == "" {
		s = defaultValue
	}
	return s
}
