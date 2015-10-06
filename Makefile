PROJECT := load-balancer
SCRIPTDIR := $(shell pwd)
ROOTDIR := $(shell cd $(SCRIPTDIR) && pwd)
VERSION:= $(shell cat $(ROOTDIR)/VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

GOBUILDDIR := $(SCRIPTDIR)/.gobuild
SRCDIR := $(SCRIPTDIR)
BINDIR := $(ROOTDIR)

ORGPATH := arvika.pulcy.com/pulcy
ORGDIR := $(GOBUILDDIR)/src/$(ORGPATH)
REPONAME := $(PROJECT)
REPODIR := $(ORGDIR)/$(REPONAME)
REPOPATH := $(ORGPATH)/$(REPONAME)
BIN := $(BINDIR)/$(PROJECT)

GOPATH := $(GOBUILDDIR)

ifndef GOOS
	GOOS := $(shell go env GOOS)
endif
ifndef GOARCH
	GOARCH := $(shell go env GOARCH)
endif	

SOURCES := $(shell find $(SRCDIR) -name '*.go')

.PHONY: all clean deps 

all: $(BIN) $(BINGPG)

clean:
	rm -Rf $(BIN) $(BINGPG) $(GOBUILDDIR)

deps: 
	@${MAKE} -B -s $(GOBUILDDIR) $(GOBINDATA)

$(GOBINDATA):
	GOPATH=$(GOPATH) go get github.com/jteeuwen/go-bindata/...
	
$(GOBUILDDIR): 
	@mkdir -p $(ORGDIR)
	@rm -f $(REPODIR) && ln -s ../../../.. $(REPODIR)
	@cd $(GOPATH) && pulcy go get github.com/dchest/uniuri
	@cd $(GOPATH) && pulcy go get github.com/juju/errgo
	@cd $(GOPATH) && pulcy go get github.com/op/go-logging
	@cd $(GOPATH) && pulcy go get github.com/spf13/cobra
	@cd $(GOPATH) && pulcy go get github.com/spf13/pflag
	@cd $(GOPATH) && pulcy go get github.com/coreos/go-etcd/etcd

$(BIN): $(GOBUILDDIR) $(SOURCES) 
	docker run \
	    --rm \
	    -v $(ROOTDIR):/usr/code \
	    -e GOPATH=/usr/code/.gobuild \
	    -e GOOS=$(GOOS) \
	    -e GOARCH=$(GOARCH) \
	    -w /usr/code/ \
	    golang:1.4.2-cross \
	    go build -a -ldflags "-X main.projectVersion $(VERSION) -X main.projectBuild $(COMMIT)" -o /usr/code/$(PROJECT)

