package main

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	namespace = "mesos"
	subsystem = "task"
)

var labels = []string{"executor_id", "framework", "task"}

type MonitoredTask struct {
	ExecutorId  string `json:"executor_id"`
	FrameworkId string `json:"framework_id"`
	Statistics  Statistics
}

type Statistics struct {
	CpusLimit          float64 `json:"cpus_limit"`
	CpusSystemTimeSecs float64 `json:"cpus_system_time_secs"`
	CpusUserTimeSecs   float64 `json:"cpus_user_time_secs"`
	MemLimitBytes      int64   `json:"mem_limit_bytes"`
	MemRssBytes        int64   `json:"mem_rss_bytes"`
	Timestamp          float64
}

type taskMetric struct {
	frameworkName string
	taskName      string
}

func findTaskName(executorId string, framework Framework) string {
	for _, task := range framework.Tasks {
		if task.Id == executorId {
			return task.Name
		}
	}

	return ""
}

func newCounterVec(constLabels prometheus.Labels, help string, name string) *prometheus.CounterVec {
	counterVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			ConstLabels: constLabels,
			Help:        help,
			Name:        name,
			Namespace:   namespace,
			Subsystem:   subsystem,
		},
		labels)

	prometheus.MustRegister(counterVec)

	return counterVec
}

func newGaugeVec(constLabels prometheus.Labels, help string, name string) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			ConstLabels: constLabels,
			Help:        help,
			Name:        name,
			Namespace:   namespace,
			Subsystem:   subsystem,
		},
		labels)

	prometheus.MustRegister(gaugeVec)

	return gaugeVec
}

func retrieveStats(c *http.Client, stats *[]MonitoredTask, url string) error {
	resp, err := c.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, stats)
	if err != nil {
		return err
	}

	return nil
}

// Periodically queries a Mesos slave and updates statistics of each running task
func slavePoller(c *http.Client, conf *Config, frameworkRegistry *frameworkRegistry, slave Slave, erroredSlaves *map[string]struct{}) {
	var knownTasks map[string]taskMetric
	var monitoredTasks []MonitoredTask

	knownTasks = make(map[string]taskMetric)

	slaveStatsUrl := fmt.Sprintf("http://%s/monitor/statistics.json", slave.address())

	constLabels := prometheus.Labels{"slave_pid": slave.Pid}

	cpusLimitGauge := newGaugeVec(
		constLabels,
		"CPU limit of the task.",
		"cpus_limit",
	)

	cpusSystemTimeCounter := newCounterVec(
		constLabels,
		"Absolute CPU sytem time.",
		"cpus_system_time_seconds",
	)

	cpusUserTimeCounter := newCounterVec(
		constLabels,
		"Absolute CPU user time.",
		"cpus_user_time_seconds",
	)

	memLimitGauge := newGaugeVec(
		constLabels,
		"Maximum memory available to the task.",
		"mem_limit_bytes",
	)

	memRssGauge := newGaugeVec(
		constLabels,
		"Current Memory usage.",
		"mem_rss_bytes",
	)

	t := time.Tick(conf.MesosSlaveQueryInterval)

	for _ = range t {
		log.Debugf("Scraping slave '%s'", slave.Pid)

		availableTasks := make(map[string]struct{})

		err := retrieveStats(c, &monitoredTasks, slaveStatsUrl)
		if err != nil {
			prometheus.Unregister(cpusLimitGauge)
			prometheus.Unregister(cpusSystemTimeCounter)
			prometheus.Unregister(cpusUserTimeCounter)
			prometheus.Unregister(memLimitGauge)
			prometheus.Unregister(memRssGauge)

			log.Errorf("Error retrieving stats from slave '%s' - Stopping goroutine", slave.Pid)

			(*erroredSlaves)[slave.Pid] = struct{}{}
			return
		}

		for _, item := range monitoredTasks {
			var frameworkName string
			var taskName string

			availableTasks[item.ExecutorId] = struct{}{}

			cpusLimit := item.Statistics.CpusLimit
			cpusSystemTime := item.Statistics.CpusSystemTimeSecs
			cpusUserTime := item.Statistics.CpusUserTimeSecs
			memLimit := float64(item.Statistics.MemLimitBytes)
			memRss := float64(item.Statistics.MemRssBytes)

			metric, ok := knownTasks[item.ExecutorId]
			if ok {
				frameworkName = metric.frameworkName
				taskName = metric.taskName
			} else {
				framework, err := frameworkRegistry.Get(item.FrameworkId)
				if err != nil {
					log.Debugf("Framework '%s' of task '%s' not registered - not scraping", item.FrameworkId, item.ExecutorId)
					continue
				}

				frameworkName = framework.Name
				taskName = findTaskName(item.ExecutorId, framework)

				if taskName == "" {
					log.Debugf("Could not find name of task of executor '%s' - skipping", item.ExecutorId)
					continue
				}

				log.Debugf("Found new task '%s'", item.ExecutorId)

				knownTasks[item.ExecutorId] = taskMetric{
					frameworkName: frameworkName,
					taskName:      taskName,
				}
			}

			cpusLimitGauge.WithLabelValues(item.ExecutorId, frameworkName, taskName).Set(cpusLimit)

			cpusSystemTimeCounter.WithLabelValues(item.ExecutorId, frameworkName, taskName).Set(cpusSystemTime)

			cpusUserTimeCounter.WithLabelValues(item.ExecutorId, frameworkName, taskName).Set(cpusUserTime)

			memLimitGauge.WithLabelValues(item.ExecutorId, frameworkName, taskName).Set(memLimit)

			memRssGauge.WithLabelValues(item.ExecutorId, frameworkName, taskName).Set(memRss)
		}

		// Remove tasks that have finished since the last check and unregister the metrics associated with the task
		for executorId, metric := range knownTasks {
			_, ok := availableTasks[executorId]
			if ok == false {
				log.Debugf("Removing finished task '%s'", executorId)

				cpusLimitGauge.DeleteLabelValues(executorId, metric.frameworkName, metric.taskName)
				cpusSystemTimeCounter.DeleteLabelValues(executorId, metric.frameworkName, metric.taskName)
				cpusUserTimeCounter.DeleteLabelValues(executorId, metric.frameworkName, metric.taskName)
				memLimitGauge.DeleteLabelValues(executorId, metric.frameworkName, metric.taskName)
				memRssGauge.DeleteLabelValues(executorId, metric.frameworkName, metric.taskName)

				delete(knownTasks, executorId)
			}
		}
	}
}
