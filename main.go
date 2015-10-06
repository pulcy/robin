package main

import (
	"fmt"
	"os"

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
)

func init() {
	logging.SetFormatter(logging.MustStringFormatter("[%{level:-5s}] %{message}"))
	cmdMain.Run = cmdMainRun
}

func main() {
	cmdMain.Execute()
}

func cmdMainRun(cmd *cobra.Command, args []string) {
	// Prepare service
	config := service.ServiceConfig{}
	deps := service.ServiceDependencies{
		Logger: log,
	}
	service := service.NewService(config, deps)
	service.Run()
}

func Exitf(format string, args ...interface{}) {
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
