# Load-balancer
FROM alpine:3.2

# ---------------------------------------------------------
# Installation
# ---------------------------------------------------------

# Install curl, supervisor & haproxy
RUN apk add -U haproxy curl supervisor

# Install confd
ENV CONFD_VERSION 0.10.0
RUN curl -o /usr/local/bin/confd https://github.com/kelseyhightower/confd/releases/download/v${CONFD_VERSION}/confd-${CONFD_VERSION}-darwin-amd64
RUN chmod 0755 /usr/local/bin/confd

RUN mkdir -p /etc/confd/conf.d
RUN mkdir -p /etc/confd/templates

# ---------------------------------------------------------
# Configuration
# ---------------------------------------------------------

# Configure haproxy
RUN mkdir -p /data/logs
RUN mkdir -p /data/config
RUN mkdir -p /data/config
RUN mkdir -p /var/lib/haproxy/dev

# Add files
ADD ./errors/ /app/errors/
ADD ./public_html/ /app/public_html/
ADD ./supervisord.conf /etc/supervisord.conf
ADD ./conf.d/haproxy-iggi.toml /etc/confd/conf.d/haproxy-iggi.toml
ADD ./templates/haproxy-iggi.tmpl /etc/confd/templates/haproxy-iggi.tmpl
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
VOLUME ["/dev/log"]

# Export ports
EXPOSE 80
EXPOSE 443
EXPOSE 7086

# Start supervisord
CMD ["/app/start.sh"]
