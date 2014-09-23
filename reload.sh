#!/bin/bash

# Reload nginx
pid=$(cat /nginx.pid)
echo "Reloading nginx ($pid)"
kill -HUP $pid

