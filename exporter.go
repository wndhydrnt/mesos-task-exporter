package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
)

type Exporter struct {
	config            *Config
	frameworkRegistry *frameworkRegistry
	httpClient        *http.Client
}

func (e *Exporter) Run() {
	http.Handle(e.config.ExporterEndpoint, prometheus.Handler())

	go http.ListenAndServe(e.config.ExporterAddress, nil)

	mp := masterPoller{
		config:            e.config,
		frameworkRegistry: e.frameworkRegistry,
		httpClient:        e.httpClient,
	}

	go mp.run()
}

func NewExporter(config *Config) *Exporter {
	c := &http.Client{}

	return &Exporter{
		config:            config,
		frameworkRegistry: NewFrameworkRegistry(),
		httpClient:        c,
	}
}
