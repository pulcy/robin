# Load-balancer
FROM ewoutp/docker-nginx-confd

# Configure nginx
ADD ./nginx.conf /etc/nginx.conf
RUN mkdir -p /data/logs
RUN mkdir -p /data/config

# Add files
ADD ./conf.d/nginx-subliminl.toml /etc/confd/conf.d/nginx-subliminl.toml
ADD ./templates/nginx-subliminl.tmpl /etc/confd/templates/nginx-subliminl.tmpl
ADD ./start.sh /start.sh
ADD ./reload.sh /reload.sh
ADD ./public_html/502.html /public_html/502.html
ADD ./public_html/index.html /usr/local/nginx/html/index.html

# Configure volumns
VOLUME ["/data"]
EXPOSE 80
EXPOSE 443

# Configure startup
ADD ./supervisord.conf /etc/supervisord.conf

# Start supervisord
CMD ["/start.sh"]
