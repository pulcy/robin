#!/bin/bash

# Ensure data directories exist
mkdir -p /data/cache
mkdir -p /data/config
mkdir -p /data/logs
mkdir -p /data/tls

# Prepare environment
export ETCD_URL=http://$ETCD_PORT_4001_TCP_ADDR:$ETCD_PORT_4001_TCP_PORT
echo "ETCD_URL=$ETCD_URL"

# Start supervisord
/usr/bin/supervisord -c /etc/supervisord.conf
