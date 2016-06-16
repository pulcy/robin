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
GOVERSION := 1.6.2-alpine

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
		github.com/pulcy/registrator-api \
		github.com/pulcy/robin-api \
		github.com/spf13/cobra \
		github.com/spf13/pflag \
		github.com/xenolf/lego

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
