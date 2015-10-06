package service

import (
	"net/url"
	"path"

	"github.com/coreos/go-etcd/etcd"
	"github.com/op/go-logging"
)

const (
	servicePrefix = "service"
)

type etcdBackend struct {
	client    *etcd.Client
	waitIndex uint64
	Logger    *logging.Logger
	prefix    string
}

func NewEtcdBackend(logger *logging.Logger, uri *url.URL) Backend {
	urls := make([]string, 0)
	if uri.Host != "" {
		urls = append(urls, "http://"+uri.Host)
	}
	return &etcdBackend{
		client: etcd.NewClient(urls),
		prefix: uri.Path,
		Logger: logger,
	}
}

// Watch for changes on a path and return where there is a change.
func (eb *etcdBackend) Watch() error {
	resp, err := eb.client.Watch(eb.prefix, eb.waitIndex, true, nil, nil)
	if err != nil {
		return maskAny(err)
	} else {
		eb.waitIndex = resp.EtcdIndex + 1
		return nil
	}
}

// Load all registered services
func (eb *etcdBackend) Services() ([]ServiceRegistration, error) {
	etcdPath := path.Join(eb.prefix, servicePrefix)
	sort := false
	recursive := true
	resp, err := eb.client.Get(etcdPath, sort, recursive)
	if err != nil {
		return nil, maskAny(err)
	}
	list := []ServiceRegistration{}
	if resp.Node == nil {
		return list, nil
	}
	for _, serviceNode := range resp.Node.Nodes {
		sr := ServiceRegistration{Name: path.Base(serviceNode.Key)}
		for _, backendNode := range serviceNode.Nodes {
			sr.Backends = append(sr.Backends, backendNode.Value)
		}
	}

	return list, nil

}
