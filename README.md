Subliminl Load-balancer
=======================

This project contains the source files and scripts to build the Subliminl load balancer containers.

Building
========

```
make docker
```

Running
=======

```
docker run --link <etcd-container>:etcd -v <data-dir>:/data -p 80:80 -p 443:443 load-balancer
```

Volumes
=======

This image requires 1 volume map.
* /data: Map to a directory where persistent storage is placed.

In the /data folder there are a few sub-directories:

* /data/cache: Contains nginx cache data
* /data/config: Contains config files
* /data/logs: Contains confd and nginx log files
* /data/tls: Must contain server.crt and server.key, the SSL certificate files used by nginx.

Internals
=========

This image contains 2 services:
1) confd: Listens for changes in the etcd namespace and updates a nginx configuration file accordingly.
2) nginx: Configured to run as reverse proxy and SSL terminator.

