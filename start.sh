#!/bin/bash

# Ensure data directories exist
mkdir -p /data/cache
mkdir -p /data/config
mkdir -p /data/logs
mkdir -p /data/tls

apt-get update
apt-get curl

# Prepare environment
export ETCD_URL=$ETCD_PORT_4001_TCP_ADDR:$ETCD_PORT_4001_TCP_PORT
echo "ETCD_URL=$ETCD_URL"
curl -L http://$ETCD_URL/v1/keys/

# Start supervisord
/usr/bin/supervisord -c /etc/supervisord.conf
