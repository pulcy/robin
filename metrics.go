package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/pulcy/macaron-utils"
	"gopkg.in/macaron.v1"
)

func startMetricsListener(host string, port int) {
	handler, err := setupMetricsRoutes(projectName, projectVersion, projectBuild)
	if err != nil {
		Exitf("Failed to setup routes: %#v", err)
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	fmt.Printf("Starting %s (version %s build %s) on %s\n", projectName, projectVersion, projectBuild, addr)
	go func() {
		if err := http.ListenAndServe(addr, handler); err != nil {
			Exitf("ListenAndServe failed: %#v", err)
		}
	}()
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
