package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"net/url"
	"strings"
	"time"
)

var (
	exporterAddress          = flag.String("exporter.address", ":55555", "Address of the exporter")
	exporterEndpoint         = flag.String("exporter.endpoint", "/metrics", "Path where metrics are served")
	logLevel                 = flag.String("log.level", "info", "Log level")
	mesosMasters             = flag.String("mesos.masters", "http://localhost:5050", "A list of Mesos masters separated by commas")
	mesosMasterQueryInterval = flag.Duration("mesos.master-pollinterval", 15*time.Second, "Interval to poll the Mesos master leader for new slaves")
	mesosSlaveQueryInterval  = flag.Duration("mesos.slave-pollinterval", 15*time.Second, "Interval to poll a Mesos slave for stats of tasks")
)

type Config struct {
	ExporterAddress          string
	ExporterEndpoint         string
	LogLevel                 log.Level
	MesosMasters             []*url.URL
	MesosMasterQueryInterval time.Duration
	MesosSlaveQueryInterval  time.Duration
}

func newConfig() *Config {
	flag.Parse()

	logLevel, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Errorf("Invalid log level '%s' - defaulting to INFO", logLevel)
		logLevel = log.InfoLevel
	}

	masterUrls := []*url.URL{}

	for _, rawUrl := range strings.Split(*mesosMasters, ",") {
		masterUrl, err := url.Parse(rawUrl)
		if err != nil {
			log.Fatalf("Unable to parse URL '%s': '%s'", rawUrl, err)
		}

		masterUrls = append(masterUrls, masterUrl)
	}

	return &Config{
		ExporterAddress:          *exporterAddress,
		ExporterEndpoint:         *exporterEndpoint,
		LogLevel:                 logLevel,
		MesosMasters:             masterUrls,
		MesosMasterQueryInterval: *mesosMasterQueryInterval,
		MesosSlaveQueryInterval:  *mesosSlaveQueryInterval,
	}
}
