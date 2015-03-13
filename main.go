package main

import (
	log "github.com/Sirupsen/logrus"
	"os"
	"os/signal"
)

func main() {
	config := newConfig()

	log.SetLevel(config.LogLevel)

	e := NewExporter(config)

	go e.Run()

	sc := make(chan os.Signal, 1)

	signal.Notify(sc, os.Interrupt, os.Kill)

	s := <-sc
	log.Infof("Received signal: %s", s)
	log.Info("Shutting down...")
}
