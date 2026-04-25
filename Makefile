GOPATH ?= $(shell go env GOPATH)
CONTROLLER_GEN = $(GOPATH)/bin/controller-gen
BOILERPLATE = hack/boilerplate.go.txt
PACKAGE = ./pkg/plugins/happyedge/...

.PHONY: generate build test clean

generate:
	$(CONTROLLER_GEN) object:headerFile="$(BOILERPLATE)" paths="$(PACKAGE)"

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/kube-scheduler ./cmd/scheduler

test:
	go test ./...

clean:
	rm -f bin/kube-scheduler