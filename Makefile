test:
	exit 0

release:
	go get github.com/prometheus/procfs
	go get github.com/mitchellh/gox
	gox -build-toolchain -os="linux"
	gox -os="linux"

.PHONY: clean
clean:
	rm -rf mesos-task-exporter_*
