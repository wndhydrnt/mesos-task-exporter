# Changelog

## 0.3.0 (unreleased)

## 0.2.2

Bug Fixes:
* Fix a regression introduced in 0.2.1 where new tasks are not picked up

## 0.2.1

Bug Fixes:
* Metrics of slaves and frameworks are not removed after deregistration

## 0.2.0

Features:
* Record metric `mesos_task_cpus_limit`
* Record global task stats
* Record resources advertised by slaves
* Record resources used by frameworks

Improvements:
* Remove calculation of `*_usage` metrics

## 0.1.2

Bug Fixes:
* Wrong master being polled after a leader election

## 0.1.1

Bug Fixes:
* Label "task" is empty ([#1](https://github.com/wndhydrnt/mesos-task-exporter/issues/1))

## 0.1.0

Initial release
