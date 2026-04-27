GOPATH ?= $(shell go env GOPATH)
CONTROLLER_GEN = $(GOPATH)/bin/controller-gen
CONVERSION_GEN = $(GOPATH)/bin/conversion-gen
DEFAULTER_GEN = $(GOPATH)/bin/defaulter-gen
BOILERPLATE = hack/boilerplate.go.txt
MODULE = github.com/montypx/happy-edge-scheduling-plugin

.PHONY: generate build test clean

generate:
	$(CONTROLLER_GEN) object:headerFile="$(BOILERPLATE)" paths="./apis/..."
	$(DEFAULTER_GEN) \
		--output-file zz_generated.defaults.go \
		--go-header-file $(BOILERPLATE) \
		$(MODULE)/apis/config/v1
	$(CONVERSION_GEN) \
		--output-file zz_generated.conversion.go \
		--go-header-file $(BOILERPLATE) \
		$(MODULE)/apis/config/v1

build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/kube-scheduler ./cmd/scheduler

test:
	go test ./...

clean:
	rm -f bin/kube-scheduler