# Load-balancer
FROM ubuntu:14.04.1

# ---------------------------------------------------------
# Installation
# ---------------------------------------------------------

# Install curl, supervisor & haproxy
#RUN mv /etc/mime.types /etc/nginx.mime.types
RUN DEBIAN_FRONTEND=noninteractive  apt-get update && apt-get install -y curl supervisor  python-software-properties software-properties-common
RUN DEBIAN_FRONTEND=noninteractive  apt-add-repository ppa:vbernat/haproxy-1.5
RUN DEBIAN_FRONTEND=noninteractive  apt-get update
RUN DEBIAN_FRONTEND=noninteractive  apt-get install -y haproxy
RUN DEBIAN_FRONTEND=noninteractive  apt-get install -y nano
RUN DEBIAN_FRONTEND=noninteractive  apt-get clean

# Install confd
ENV CONFD_VERSION 0.6.3

ADD ./bin/confd /usr/local/bin/confd
RUN chmod 0755 /usr/local/bin/confd
RUN mkdir -p /etc/confd/conf.d
RUN mkdir -p /etc/confd/templates

# ---------------------------------------------------------
# Configuration
# ---------------------------------------------------------

# Configure haproxy
RUN mkdir -p /data/logs
RUN mkdir -p /data/config

# Add files
ADD ./errors/ /app/errors/
ADD ./public_html/ /app/public_html/
ADD ./supervisord.conf /etc/supervisord.conf
ADD ./conf.d/haproxy-subliminl.toml /etc/confd/conf.d/haproxy-subliminl.toml
ADD ./templates/haproxy-subliminl.tmpl /etc/confd/templates/haproxy-subliminl.tmpl
ADD ./start.sh /app/start.sh
ADD ./reload-haproxy.sh /app/reload-haproxy.sh

# Create error responses
RUN cat /app/errors/400.hdr /app/public_html/400.html > /app/errors/400.http
RUN cat /app/errors/403.hdr /app/public_html/403.html > /app/errors/403.http
RUN cat /app/errors/408.hdr /app/public_html/408.html > /app/errors/408.http
RUN cat /app/errors/500.hdr /app/public_html/500.html > /app/errors/500.http
RUN cat /app/errors/502.hdr /app/public_html/50x.html > /app/errors/502.http
RUN cat /app/errors/503.hdr /app/public_html/50x.html > /app/errors/503.http
RUN cat /app/errors/504.hdr /app/public_html/50x.html > /app/errors/504.http

# Configure volumns
VOLUME ["/data"]
EXPOSE 80
EXPOSE 443

# Configure startup
ADD ./supervisord.conf /etc/supervisord.conf

# Start supervisord
CMD ["/app/start.sh"]
