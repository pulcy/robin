PROJECT := robin
SCRIPTDIR := $(shell pwd)
ROOTDIR := $(shell cd $(SCRIPTDIR) && pwd)
VERSION:= $(shell cat $(ROOTDIR)/VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

GOBUILDDIR := $(SCRIPTDIR)/.gobuild
SRCDIR := $(SCRIPTDIR)
BINDIR := $(ROOTDIR)
VENDORDIR := $(SCRIPTDIR)/deps

ORGPATH := github.com/pulcy
ORGDIR := $(GOBUILDDIR)/src/$(ORGPATH)
REPONAME := $(PROJECT)
REPODIR := $(ORGDIR)/$(REPONAME)
REPOPATH := $(ORGPATH)/$(REPONAME)
BIN := $(BINDIR)/$(PROJECT)

GOPATH := $(GOBUILDDIR)
GOVERSION := 1.7.3-alpine

ifndef GOOS
	GOOS := linux
endif
ifndef GOARCH
	GOARCH := amd64
endif

SOURCES := $(shell find $(SRCDIR) -name '*.go')

.PHONY: all clean deps docker

all: $(BIN)

clean:
	rm -Rf $(BIN) $(GOBUILDDIR)

local:
	@${MAKE} -B GOOS=$(shell go env GOHOSTOS) GOARCH=$(shell go env GOHOSTARCH) $(BIN)

deps:
	@${MAKE} -B -s $(GOBUILDDIR)

$(GOBUILDDIR):
	@mkdir -p $(ORGDIR)
	@rm -f $(REPODIR) && ln -s ../../../.. $(REPODIR)
	@GOPATH=$(GOPATH) pulsar go flatten -V $(VENDORDIR)

update-vendor:
	@rm -Rf $(VENDORDIR)
	@pulsar go vendor -V $(VENDORDIR) \
		github.com/coreos/etcd/client \
		github.com/dchest/uniuri \
		github.com/giantswarm/retry-go \
		github.com/juju/errgo \
		github.com/mitchellh/go-homedir \
		github.com/op/go-logging \
		github.com/pulcy/macaron-utils \
		github.com/pulcy/registrator-api \
		github.com/pulcy/rest-kit \
		github.com/pulcy/robin-api \
		github.com/spf13/cobra \
		github.com/spf13/pflag \
		github.com/xenolf/lego \
		github.com/YakLabs/k8s-client \
		gopkg.in/macaron.v1

$(BIN): $(GOBUILDDIR) $(SOURCES)
	docker run \
		--rm \
		-v $(ROOTDIR):/usr/code \
		-e GOPATH=/usr/code/.gobuild \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-e CGO_ENABLED=0 \
		-w /usr/code/ \
		golang:$(GOVERSION) \
		go build -a -installsuffix netgo -tags netgo -ldflags "-X main.projectVersion=$(VERSION) -X main.projectBuild=$(COMMIT)" -o /usr/code/$(PROJECT) $(REPOPATH)

run-tests:
	@make run-test test=$(REPOPATH)/service

update-tests:
	@make run-tests UPDATE-FIXTURES=1

run-test:
	@if test "$(test)" = "" ; then \
		echo "missing test parameter, that is, path to test folder e.g. './middleware/'."; \
		exit 1; \
	fi
	@docker run \
	    --rm \
	    -v $(shell pwd):/usr/code \
	    -e GOPATH=/usr/code/.gobuild \
		-e TEST_ENV=test-env \
		-e UPDATE-FIXTURES=$(UPDATE-FIXTURES) \
	    -w /usr/code \
		golang:$(GOVERSION) \
	    go test -v $(test)
