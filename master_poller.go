package main

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Framework struct {
	Active bool
	Id     string
	Name   string
	Tasks  []Task
}

type Master struct {
	Frameworks []Framework
	Leader     string
	Slaves     []Slave
}

type Slave struct {
	Pid string
}

func (s *Slave) address() string {
	parts := strings.Split(s.Pid, "@")
	return parts[1]
}

type Task struct {
	Id   string
	Name string
}

type masterPoller struct {
	config             *Config
	currentMesosMaster *url.URL
	frameworkRegistry  *frameworkRegistry
	httpClient         *http.Client
}

// Periodically queries a Mesos master to check for new slaves.
func (e *masterPoller) run() {
	knownSlaves := make(map[string]struct{})

	e.poll(knownSlaves)

	t := time.Tick(e.config.MesosMasterQueryInterval)

	for _ = range t {
		e.poll(knownSlaves)
	}
}

func (e *masterPoller) retrieveCurrentMasterState() (Master, error) {
	if e.currentMesosMaster != nil {
		var master Master

		err := retrieveMasterState(e.httpClient, &master, e.currentMesosMaster.String())
		if err != nil {
			log.Errorf("Unable to retrieve data from Mesos master '%s': %s", e.currentMesosMaster.String(), err)
		} else {
			return master, nil
		}
	}

	for _, masterUrl := range e.config.MesosMasters {
		var master Master

		err := retrieveMasterState(e.httpClient, &master, masterUrl.String())
		if err != nil {
			log.Errorf("Unable to retrieve data from Mesos master '%s': %s", masterUrl, err)
			continue
		}

		leaderParts := strings.Split(master.Leader, "@")
		if leaderParts[1] == masterUrl.Host {
			log.Infof("Detected '%s' as the current Mesos master leader", master.Leader)
			e.currentMesosMaster = masterUrl
			return master, nil
		}
	}

	return Master{}, errors.New("Unable to retrieve current Master state")
}

func (e *masterPoller) poll(knownSlaves map[string]struct{}) {
	availableSlaves := make(map[string]struct{})

	master, err := e.retrieveCurrentMasterState()
	if err != nil {
		log.Error(err)
		return
	}

	for _, framework := range master.Frameworks {
		e.frameworkRegistry.Set(framework)
	}

	// Start reading stats of a new slave.
	for _, slave := range master.Slaves {
		availableSlaves[slave.Pid] = struct{}{}

		_, ok := knownSlaves[slave.Pid]
		if ok == false {
			log.Debugf("Scraping slave '%s'", slave.Pid)
			knownSlaves[slave.Pid] = struct{}{}
			go slavePoller(e.httpClient, e.config, e.frameworkRegistry, slave)
		}
	}

	// Remove slaves that have gone offline.
	for knownSlave, _ := range knownSlaves {
		_, ok := availableSlaves[knownSlave]

		if ok == false {
			log.Debugf("Removing slave '%s'", knownSlave)
			delete(knownSlaves, knownSlave)
		}
	}
}

func retrieveMasterState(c *http.Client, master *Master, url string) error {
	masterUrl := url + "/master/state.json"

	resp, err := c.Get(masterUrl)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, master)
	if err != nil {
		return err
	}

	return nil
}
