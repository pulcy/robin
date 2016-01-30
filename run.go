package main

import (
	"net/url"
	"path"

	"github.com/coreos/go-etcd/etcd"
	"github.com/spf13/cobra"

	"git.pulcy.com/pulcy/load-balancer/service"
	"git.pulcy.com/pulcy/load-balancer/service/acme"
	"git.pulcy.com/pulcy/load-balancer/service/backend"
	"git.pulcy.com/pulcy/load-balancer/service/locks"
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
		etcdAddr        string
		haproxyConfPath string
		statsPort       int
		statsUser       string
		statsPassword   string
		statsSslCert    string
		sslCertsFolder  string
		forceSsl        bool
		privateHost     string

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
	cmdRun.Flags().StringVar(&runArgs.etcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdRun.Flags().StringVar(&runArgs.haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
	cmdRun.Flags().IntVar(&runArgs.statsPort, "stats-port", defaultStatsPort, "Port for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsUser, "stats-user", defaultStatsUser, "User for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsPassword, "stats-password", defaultStatsPassword, "Password for stats page")
	cmdRun.Flags().StringVar(&runArgs.statsSslCert, "stats-ssl-cert", defaultStatsSslCert, "Filename of SSL certificate for stats page (located in ss-certs)")
	cmdRun.Flags().StringVar(&runArgs.sslCertsFolder, "ssl-certs", defaultSslCertsFolder, "Folder containing SSL certificate")
	cmdRun.Flags().BoolVar(&runArgs.forceSsl, "force-ssl", defaultForceSsl, "Redirect HTTP to HTTPS")
	cmdRun.Flags().StringVar(&runArgs.privateHost, "private-host", defaultPrivateHost, "IP address of private network")
	// acme
	cmdRun.Flags().IntVar(&runArgs.acmeHttpPort, "acme-http-port", defaultAcmeHttpPort, "Port to listen for ACME HTTP challenges on (internally)")
	cmdRun.Flags().StringVar(&runArgs.acmeEmail, "acme-email", "", "Email account for ACME server")
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
	etcdClient := etcd.NewClient([]string{"http://" + etcdUrl.Host})

	// Prepare backend
	backend := backend.NewEtcdBackend(log, etcdUrl)

	// Prepare lockservice
	lockService := locks.NewEtcdLockService(etcdClient, path.Join(etcdUrl.Path, etcdLocksFolder))

	// Prepare acme service
	acmeEtcdPrefix := path.Join(etcdUrl.Path, etcdAcmeFolder)
	acmeServiceListener := &acmeServiceListener{}
	acmeService := acme.NewAcmeService(acme.AcmeServiceConfig{
		HttpProviderConfig: acme.HttpProviderConfig{
			EtcdPrefix: acmeEtcdPrefix,
			Port:       runArgs.acmeHttpPort,
		},
		EtcdPrefix:         acmeEtcdPrefix,
		CADirectoryURL:     runArgs.caDirURL,
		KeyBits:            runArgs.keyBits,
		Email:              runArgs.acmeEmail,
		PrivateKeyPath:     runArgs.privateKeyPath,
		RegistrationPath:   runArgs.registrationPath,
		TmpCertificatePath: runArgs.tmpCertificatePath,
	}, acme.AcmeServiceDependencies{
		HttpProviderDependencies: acme.HttpProviderDependencies{
			Logger:     log,
			EtcdClient: etcdClient,
		},
		LockService: lockService,
		Listener:    acmeServiceListener,
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
