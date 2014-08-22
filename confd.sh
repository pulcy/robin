#!/bin/bash

export URL=http://$ETCD_PORT_4001_TCP_ADDR:$ETCD_PORT_4001_TCP_PORT
echo Using etcd node: $URL
/usr/local/bin/confd -node $URL -interval=30
