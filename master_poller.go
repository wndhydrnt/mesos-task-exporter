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
	Active        bool
	Id            string
	Name          string
	Tasks         []Task
	UsedResources Resources `json:"used_resources"`
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

type Resources struct {
	Cpus  float64
	Disk  float64
	Mem   float64
	Ports string
}

type Slave struct {
	Pid       string
	Resources Resources
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
	frameworkResources *prometheus.GaugeVec
	frameworkRegistry  *frameworkRegistry
	httpClient         *http.Client
	slaveResources     *prometheus.GaugeVec
	tasksCounterVec    *prometheus.CounterVec
}

// Periodically queries a Mesos master to check for new slaves.
func (e *masterPoller) run() {
	knownSlaves := make(map[string]struct{})

	e.frameworkResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "Resources assigned to a framework",
			Name:      "resources",
			Namespace: "mesos",
			Subsystem: "framework",
		},
		[]string{"name", "resource", "type"})
	prometheus.MustRegister(e.frameworkResources)

	e.slaveResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Help:      "Resources advertised by a slave",
			Name:      "resources",
			Namespace: "mesos",
			Subsystem: "slave",
		},
		[]string{"pid", "resource"})
	prometheus.MustRegister(e.slaveResources)

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

	e.handleFrameworks(master.Frameworks, e.frameworkResources)

	// Start reading stats of a new slave.
	for _, slave := range master.Slaves {
		availableSlaves[slave.Pid] = struct{}{}

		e.slaveResources.WithLabelValues(slave.Pid, "cpus").Set(slave.Resources.Cpus)
		e.slaveResources.WithLabelValues(slave.Pid, "disk").Set(slave.Resources.Disk)
		e.slaveResources.WithLabelValues(slave.Pid, "mem").Set(slave.Resources.Mem)

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

			e.slaveResources.DeleteLabelValues(knownSlave, "cpus")
			e.slaveResources.DeleteLabelValues(knownSlave, "disk")
			e.slaveResources.DeleteLabelValues(knownSlave, "mem")

			delete(knownSlaves, knownSlave)
		}
	}
}

func (e *masterPoller) handleFrameworks(frameworks []Framework, resources *prometheus.GaugeVec) {
	knownFrameworks := e.frameworkRegistry.All()

	availableFrameworks := make(map[string]struct{})

	for _, framework := range frameworks {
		e.frameworkResources.WithLabelValues(framework.Name, "cpus", "used").Set(framework.UsedResources.Cpus)
		e.frameworkResources.WithLabelValues(framework.Name, "disk", "used").Set(framework.UsedResources.Disk)
		e.frameworkResources.WithLabelValues(framework.Name, "mem", "used").Set(framework.UsedResources.Mem)

		availableFrameworks[framework.Id] = struct{}{}

		_, ok := knownFrameworks[framework.Id]
		if ok == false {
			e.frameworkRegistry.Set(framework)
		}
	}

	for _, knownFramework := range knownFrameworks {
		_, ok := availableFrameworks[knownFramework.Id]
		if ok == false {
			e.frameworkResources.DeleteLabelValues(knownFramework.Name, "cpus", "used")
			e.frameworkResources.DeleteLabelValues(knownFramework.Name, "disk", "used")
			e.frameworkResources.DeleteLabelValues(knownFramework.Name, "mem", "used")
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
