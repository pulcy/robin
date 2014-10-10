#!/bin/bash

# Reload haproxy
echo "Reloading haproxy"
haproxy -D -f /data/config/haproxy.cfg -p /var/run/haproxy.pid -st $(cat /var/run/haproxy.pid)
