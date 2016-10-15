PROJECT = robin-api
SCRIPTDIR := $(shell pwd)
GOBUILDDIR := $(SCRIPTDIR)/.gobuild
ORGPATH := github.com/pulcy
ORGDIR := $(GOBUILDDIR)/src/$(ORGPATH)
REPODIR := $(ORGDIR)/$(PROJECT)

all: $(GOBUILDDIR)
	GOPATH=$(GOBUILDDIR) go build .

clean:
	rm -Rf $(GOBUILDDIR)

test: $(GOBUILDDIR)
	GOPATH=$(GOBUILDDIR) go test ./...

$(GOBUILDDIR):
	@mkdir -p $(ORGDIR)
	@rm -f $(REPODIR) && ln -s ../../../.. $(REPODIR)
	@GOPATH=$(GOBUILDDIR) pulsar go flatten -V deps

update-vendor:
	pulsar go vendor -V deps \
		github.com/juju/errgo \
		github.com/pulcy/rest-kit 