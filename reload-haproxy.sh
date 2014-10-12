#!/bin/bash

create_lock_or_wait () {
  path="$1"
  wait_time="${2:-10}"
  while true; do
        if mkdir "${path}.lock.d"; then
           break;
        fi
        sleep $wait_time
  done
}

remove_lock () {
  path="$1"
  rmdir "${path}.lock.d"
}

# Create a mutex to prevent multiple haproxy instances
echo "Creating lock"
create_lock_or_wait /var/run/haproxy

# Reload haproxy
echo "Reloading haproxy"
PIDS=$(cat /var/run/haproxy.pid)
haproxy -D -f /data/config/haproxy.cfg -p /var/run/haproxy.pid -st $PIDS

# Wait for old processes to terminate
if [ ! -z "$PIDS" ]; then
	counter=0
	echo "Waiting for old instances to terminate: $PIDS"
	while ps -p $PIDS; do
		if [[ "$counter" -gt 20 ]]; then
			echo "Killing processes"
			kill -9 $PIDS
		else
			counter=$((counter+1))
			echo "..."
			sleep 1
		fi
	done
fi

echo "Removing lock"
remove_lock /var/run/haproxy
