# DEPRECATED
This repository is now longer maintained. Please use [mesos_exporter](https://github.com/prometheus/mesos_exporter).

# mesos-task-exporter

Export metrics of tasks running on [Apache Mesos](http://mesos.apache.org/) for [Prometheus](http://prometheus.io/) consumption.

## Features

* Auto-discover of new slaves
* Auto-discover of new tasks
* Stateless - can run anywhere
* Framework-independent

## Metrics

### Task

#### Exported metrics

* `mesos_task_cpus_limit`
* `mesos_task_cpus_system_time_seconds`
* `mesos_task_cpus_user_time_seconds`
* `mesos_task_mem_limit_bytes`
* `mesos_task_mem_rss_bytes`

#### Labels

Every metric has the following labels attached to it:

* `executor_id` - The unique ID of the executor
* `framework` - The name of the framework that spawned the task
* `slave_pid` - The PID of the Mesos slave that the task is running on as exposed by the `/master/state.json` endpoint of the Mesos master
* `task` - The name of the task as in the Mesos UI

#### Examples

[Chronos](https://github.com/mesos/chronos):

```
mesos_task_cpus_system_time_seconds{executor_id="ct:1426247880000:0:examplejob:",framework="chronos-2.3.2_mesos-0.20.1-SNAPSHOT",slave_pid="slave(1)@10.168.1.11:5051",task="ChronosTask:examplejob"} 0.02
```

[Marathon](https://github.com/mesosphere/marathon):

```
mesos_task_cpus_system_time_seconds{executor_id="com_example_redis.b8f17462-c96c-11e4-b9ff-56847afe9799",framework="marathon",slave_pid="slave(1)@10.168.1.11:5051",task="redis.example.com"} 10.71
```

### Global task stats

#### Exported metrics

`mesos_tasks`

#### Labels

`status`

#### Example

```
mesos_tasks{status="failed"} 0
mesos_tasks{status="finished"} 1
mesos_tasks{status="killed"} 0
mesos_tasks{status="lost"} 0
mesos_tasks{status="staged"} 2
mesos_tasks{status="started"} 0
```

### Resources advertised by a Mesos slave

#### Exported metrics

`mesos_slave_resources`

#### Labels

* `pid` - The unqiue PID of the slave in the Mesos cluster
* `resource` - The name of the resource

#### Example

```
mesos_slave_resources{pid="slave(1)@10.168.1.10:5051",resource="cpus"} 2
mesos_slave_resources{pid="slave(1)@10.168.1.10:5051",resource="disk"} 35164
mesos_slave_resources{pid="slave(1)@10.168.1.10:5051",resource="mem"} 748
```

### Resources used by a framework

#### Exported metrics

`mesos_framework_resources`

#### Labels

* `name` - The name of the framework
* `resource` - The name of the resource
* `type` - The type of the metric. Currently only `used` is implemented.

#### Example

```
mesos_framework_resources{name="marathon",resource="cpus",type="used"} 0.2
mesos_framework_resources{name="marathon",resource="disk",type="used"} 0
mesos_framework_resources{name="marathon",resource="mem",type="used"} 256
```

## Configuration

```
$ ./mesos-task-exporter -h
Usage of ./mesos-task-exporter:
  -exporter.address=":55555": Address of the exporter
  -exporter.endpoint="/metrics": Path where metrics are served
  -log.level="info": Log level
  -mesos.master-pollinterval=15s: Interval to poll the Mesos master leader for new slaves
  -mesos.masters="http://localhost:5050": A list of Mesos masters separated by commas
  -mesos.slave-pollinterval=15s: Interval to poll a Mesos slave for stats of tasks
```

```
# prometheus.yml
scrape_configs:
  - job_name: mesos_task_exporter
    scrape_interval: 15s

    target_groups:
      - targets: ['localhost:55555/metrics']
```

## Deployment

Run `mesos-task-exporter` as a Docker container:

```
docker run -p 55555:55555 -e "MESOS_MASTERS=..." wandhydrant/mesos-task-exporter
```
