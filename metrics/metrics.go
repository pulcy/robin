package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/pulcy/macaron-utils"
	"gopkg.in/macaron.v1"
)

const (
	haProxyTimeout = time.Second * 5
)

type MetricsConfig struct {
	ProjectName    string
	ProjectVersion string
	ProjectBuild   string

	Host          string
	Port          int
	HaproxyCSVURI string
}

func StartMetricsListener(config MetricsConfig, log *logging.Logger) error {
	if config.HaproxyCSVURI != "" {
		exporter, err := NewExporter(log, config.HaproxyCSVURI, serverMetrics, haProxyTimeout)
		if err != nil {
			return maskAny(err)
		}
		prometheus.MustRegister(exporter)
	} else {
		log.Info("Skipping HAProxy CSV stats: no HaproxyCSVURI configured")
	}

	handler, err := setupMetricsRoutes(config.ProjectName, config.ProjectVersion, config.ProjectBuild)
	if err != nil {
		return maskAny(fmt.Errorf("Failed to setup metrics routes: %#v", err))
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	log.Infof("Starting %s metrics (version %s build %s) on %s\n", config.ProjectName, config.ProjectVersion, config.ProjectBuild, addr)
	go func() {
		if err := http.ListenAndServe(addr, handler); err != nil {
			log.Errorf("Metrics ListenAndServe failed: %#v", err)
		}
	}()

	return nil
}

// setupMetricsRoutes prepares all routes for reading metrics.
func setupMetricsRoutes(projectName, projectVersion, projectBuild string) (http.Handler, error) {
	m := macaron.New()
	m.Use(macaron.Recovery())
	m.Use(macaron.Renderer(macaron.RenderOptions{
		// Outputs human readable JSON. Default is false.
		IndentJSON: true,
		// Outputs human readable XML. Default is false.
		IndentXML: true,
	}))

	m.SetAutoHead(true)
	m.Get("/", utils.ServerInfo(projectName, projectVersion, projectBuild))
	m.Get("/metrics", prometheus.Handler())
	/*m.Get("/rules", func(ctx *macaron.Context) {
		ctx.ServeFileContent("./rules.txt")
	})*/

	return m, nil
}
