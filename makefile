all:
	make init
	make build

init:
	go get

build:
	go build -a -tags netgo -ldflags '-extldflags "-w -static"'

clean: ./ConMAS_simple_agent
	rm ConMAS_simple_agent

containerize:
	docker build -t docker.pkg.github.com/jaist-kanno/conmas_simple_agent/simple_agent:1.0 .
