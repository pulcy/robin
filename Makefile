SCRIPTDIR := $(shell pwd)
GOBUILDDIR := $(SCRIPTDIR)/.gobuild

DOCKERTAG := $(shell devtool docker-tag)

.PHONY: all clean docker

all: bin/confd docker

clean:
	@rm -Rf bin $(GOBUILDDIR)

.gobuild/bin/confd:
	@devtool get -b v0.7.1 git@github.com:kelseyhightower/confd.git $(GOBUILDDIR)/src/github.com/kelseyhightower/confd
	@devtool get git@github.com:Subliminl/go-etcd.git $(GOBUILDDIR)/src/github.com/coreos/go-etcd
	GOPATH=$(GOBUILDDIR) go get github.com/kelseyhightower/confd
	GOPATH=$(GOBUILDDIR) go build github.com/kelseyhightower/confd

bin/confd: .gobuild/bin/confd
	@mkdir -p bin
	cp $(GOBUILDDIR)/bin/confd bin/confd

docker: bin/confd
	docker build --tag="$(DOCKERTAG)" .
