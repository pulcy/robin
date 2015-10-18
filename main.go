package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/op/go-logging"
	"github.com/spf13/cobra"

	"arvika.pulcy.com/pulcy/load-balancer/service"
)

const (
	defaultStatsPort      = 7088
	defaultStatsUser      = ""
	defaultStatsPassword  = ""
	defaultStatsSslCert   = ""
	defaultSslCertsFolder = "/certs/"
	defaultForceSsl       = false
	defaultPrivateHost    = ""
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

	etcdAddr        string
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
	cmdMain.Flags().StringVar(&etcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdMain.Flags().StringVar(&haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
	cmdMain.Flags().IntVar(&statsPort, "stats-port", defaultStatsPort, "Port for stats page")
	cmdMain.Flags().StringVar(&statsUser, "stats-user", defaultStatsUser, "User for stats page")
	cmdMain.Flags().StringVar(&statsPassword, "stats-password", defaultStatsPassword, "Password for stats page")
	cmdMain.Flags().StringVar(&statsSslCert, "stats-ssl-cert", defaultStatsSslCert, "Filename of SSL certificate for stats page (located in ss-certs)")
	cmdMain.Flags().StringVar(&sslCertsFolder, "ssl-certs", defaultSslCertsFolder, "Folder containing SSL certificate")
	cmdMain.Flags().BoolVar(&forceSsl, "force-ssl", defaultForceSsl, "Redirect HTTP to HTTPS")
	cmdMain.Flags().StringVar(&privateHost, "private-host", defaultPrivateHost, "IP address of private network")
	cmdMain.Run = cmdMainRun
}

func main() {
	cmdMain.Execute()
}

func cmdMainRun(cmd *cobra.Command, args []string) {
	// Prepare backend
	if etcdAddr == "" {
		Exitf("Please specify --etcd-addr")
	}
	etcdUrl, err := url.Parse(etcdAddr)
	if err != nil {
		Exitf("--etcd-addr '%s' is not valid: %#v", etcdAddr, err)
	}
	backend := service.NewEtcdBackend(log, etcdUrl)

	// Prepare service
	if haproxyConfPath == "" {
		Exitf("Please specify --haproxy-conf")
	}
	if privateHost == "" {
		Exitf("Please specify --private-host")
	}
	config := service.ServiceConfig{
		HaproxyConfPath: haproxyConfPath,
		StatsPort:       statsPort,
		StatsUser:       statsUser,
		StatsPassword:   statsPassword,
		StatsSslCert:    statsSslCert,
		SslCertsFolder:  sslCertsFolder,
		ForceSsl:        forceSsl,
		PrivateHost:     privateHost,
	}
	deps := service.ServiceDependencies{
		Logger:  log,
		Backend: backend,
	}
	service := service.NewService(config, deps)
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
