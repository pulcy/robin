#!/bin/bash

# Ensure data directories exist
mkdir -p /data/cache
mkdir -p /data/config
mkdir -p /data/logs
mkdir -p /data/tls

# Prepare environment
if [ -z $ETCD_URL ]; then
	export ETCD_URL=$ETCD_PORT_4001_TCP_ADDR:$ETCD_PORT_4001_TCP_PORT
fi
echo "ETCD_URL=$ETCD_URL"

# Start supervisord
/usr/bin/supervisord -c /etc/supervisord.conf
