SCRIPTDIR := $(shell pwd)
GOBUILDDIR := $(SCRIPTDIR)/.gobuild

DOCKERTAG := $(shell devtool docker-tag)

.PHONY: all clean docker

all: docker

clean:
	@rm -Rf bin $(GOBUILDDIR)

docker: 
	docker build --tag="$(DOCKERTAG)" .
