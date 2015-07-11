package main

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
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
	FailedTasks   float64 `json:"failed_tasks"`
	FinishedTasks float64 `json:"finished_tasks"`
	Frameworks    []Framework
	Leader        string
	LostTasks     float64 `json:"lost_tasks"`
	KilledTasks   float64 `json:"killed_tasks"`
	StagedTasks   float64 `json:"staged_tasks"`
	StartedTasks  float64 `json:"started_tasks"`
	Slaves        []Slave
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
	tasksCounterVec    *prometheus.CounterVec
}

// Periodically queries a Mesos master to check for new slaves.
func (e *masterPoller) run() {
	knownSlaves := make(map[string]struct{})

	e.tasksCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help:      "Cluster-wide task metrics",
			Name:      "tasks",
			Namespace: "mesos",
		},
		[]string{"status"})

	prometheus.MustRegister(e.tasksCounterVec)

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
		if err == nil {
			leaderParts := strings.Split(master.Leader, "@")
			// Only return if the elected leader has not changed
			if leaderParts[1] == e.currentMesosMaster.Host {
				return master, nil
			}
		} else {
			log.Errorf("Unable to retrieve data from Mesos master '%s': %s", e.currentMesosMaster.String(), err)
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

	e.tasksCounterVec.WithLabelValues("failed").Set(master.FailedTasks)
	e.tasksCounterVec.WithLabelValues("finished").Set(master.FinishedTasks)
	e.tasksCounterVec.WithLabelValues("killed").Set(master.KilledTasks)
	e.tasksCounterVec.WithLabelValues("lost").Set(master.LostTasks)
	e.tasksCounterVec.WithLabelValues("staged").Set(master.StagedTasks)
	e.tasksCounterVec.WithLabelValues("started").Set(master.StartedTasks)

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
