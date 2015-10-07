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
)

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:-5s}] %{message}"))
	cmdMain.Flags().StringVar(&etcdAddr, "etcd-addr", "", "Address of etcd backend")
	cmdMain.Flags().StringVar(&haproxyConfPath, "haproxy-conf", "/data/config/haproxy.cfg", "Path of haproxy config file")
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
	config := service.ServiceConfig{
		HaproxyConfPath: haproxyConfPath,
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
