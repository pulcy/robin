#!/bin/bash

# Reload nginx
pid=$(cat /nginx.pid)
echo "Reloading nginx ($pid)"
/usr/bin/supervisorctl -s unix:///supervisor.sock restart nginx
#/usr/local/sbin/nginx -s reload
#kill -HUP $pid

