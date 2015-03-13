#!/bin/bash

/go/src/github.com/wndhydrnt/mesos-task-exporter/mesos-task-exporter \
-exporter.address=$EXPORTER_ADDRESS \
-exporter.endpoint=$EXPORTER_ENDPOINT \
-log.level=$LOG_LEVEL \
-mesos.masters=$MESOS_MASTERS \
-mesos.master-pollinterval=$MESOS_MASTER_POLLINTERVAL \
-mesos.slave-pollinterval=$MESOS_SLAVE_POLLINTERVAL
