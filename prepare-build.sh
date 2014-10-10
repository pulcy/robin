#!/bin/bash

mkdir -p bin
if [ -f $GOPATH/bin/confd ]; then
	cp $GOPATH/bin/confd bin/
fi
