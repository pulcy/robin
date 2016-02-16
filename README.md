# Robin: Pulcy load-balancer

Robin is a load-balancer for HTTP, HTTPS & TCP requests.
It is configured using data in ETCD coming from [Quark](https://github.com/pulcy/quark) and
[Registrator](https://github.com/gliderlabs/registrator).

Internally, robin uses [haproxy](http://www.haproxy.org/) to do the actual load-balancing,
where the configuration for haproxy is created by robin.

Robin supports SSL connections, where you can bring your own certificate, or let robin use
[Let's Encrypt](https://letsencrypt.org/) to create certificates for you.
