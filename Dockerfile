# Load-balancer
FROM alpine:3.3

# ---------------------------------------------------------
# Installation
# ---------------------------------------------------------

# Install haproxy
RUN apk add -U haproxy curl

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

# Create error responses
RUN cat /app/errors/400.hdr /app/public_html/400.html > /app/errors/400.http
RUN cat /app/errors/403.hdr /app/public_html/403.html > /app/errors/403.http
RUN cat /app/errors/404.hdr /app/public_html/404.html > /app/errors/404.http
RUN cat /app/errors/408.hdr /app/public_html/408.html > /app/errors/408.http
RUN cat /app/errors/500.hdr /app/public_html/500.html > /app/errors/500.http
RUN cat /app/errors/502.hdr /app/public_html/50x.html > /app/errors/502.http
RUN cat /app/errors/503.hdr /app/public_html/50x.html > /app/errors/503.http
RUN cat /app/errors/504.hdr /app/public_html/50x.html > /app/errors/504.http

# Added start process
ADD ./robin /app/

# Configure volumns
VOLUME ["/data"]
VOLUME ["/dev/log"]

# Export ports
# 80:   Public HTTP
# 81:   Private HTTP
# 82:   Private TCP+SSL
# 443:  Public HTTPS
# 7088: Stats HTTPS
# 8022: Metrics
EXPOSE 80
EXPOSE 81
EXPOSE 82
EXPOSE 443
EXPOSE 7088
EXPOSE 8022

# Start the load-balancer
ENTRYPOINT ["/app/robin"]
