# Load-balancer
FROM ewoutp/docker-nginx-confd

# Configure nginx
ADD ./nginx.conf /etc/nginx.conf
RUN mkdir -p /data/logs
RUN mkdir -p /data/config

# Add files
ADD ./confd.sh /usr/local/bin/confd.sh
ADD ./conf.d/nginx-subliminl.toml /etc/confd/conf.d/nginx-subliminl.toml
ADD ./templates/nginx-subliminl.tmpl /etc/confd/templates/nginx-subliminl.tmpl

# Configure volumns
VOLUME ["/data"]

# Configure startup
ADD ./supervisord.conf /etc/supervisord.conf

# Start supervisord
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
