#!/bin/bash

# Ensure data directories exist
mkdir -p /data/cache
mkdir -p /data/config
mkdir -p /data/logs
mkdir -p /data/tls

netstat -nr | grep '^0\.0\.0\.0' | awk '{ print $2 " dockerhost" }' >> /etc/hosts

# Prepare environment
if [ -z $ETCD_URL ]; then
	export ETCD_URL=$ETCD_PORT_4001_TCP_ADDR:$ETCD_PORT_4001_TCP_PORT
fi
echo "ETCD_URL=$ETCD_URL"

# Prepare server.pem
if [ ! -f /data/tls/server.pem ]; then
	if [ -f /data/tls/server.crt ]; then
		cat /data/tls/server.crt /data/tls/server.key > /data/tls/server.pem
	fi
fi

# Set variables in configuration template
[[ -z $REGION ]] && export REGION=test
echo "REGION=$REGION"
sed -i -r "s/__REGION__/$REGION/g" /etc/confd/templates/haproxy-subliminl.tmpl

# Start supervisord
/usr/bin/supervisord -c /etc/supervisord.conf
