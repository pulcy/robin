package service

import (
	"encoding/json"
	"net/url"
	"path"

	"github.com/coreos/go-etcd/etcd"
	"github.com/op/go-logging"
)

const (
	servicePrefix  = "service"
	frontEndPrefix = "frontend"
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
		list = append(list, sr)
	}

	return list, nil
}

type frontendRecord struct {
	Selectors []frontendSelectorRecord `json:"selectors"`
	Service   string                   `json:"service,omitempty"`
}

type frontendSelectorRecord struct {
	Domain     string `json:"domain,omitempty"`
	PathPrefix string `json:"path-prefix,omitempty"`
}

// Load all registered front-ends
func (eb *etcdBackend) FrontEnds() ([]FrontEndRegistration, error) {
	etcdPath := path.Join(eb.prefix, frontEndPrefix)
	sort := false
	recursive := false
	resp, err := eb.client.Get(etcdPath, sort, recursive)
	if err != nil {
		return nil, maskAny(err)
	}
	list := []FrontEndRegistration{}
	if resp.Node == nil {
		return list, nil
	}
	for _, frontEndNode := range resp.Node.Nodes {
		rawJson := frontEndNode.Value
		record := &frontendRecord{}
		if err := json.Unmarshal([]byte(rawJson), record); err != nil {
			eb.Logger.Error("Cannot unmarshal registration of %s", frontEndNode.Key)
			continue
		}

		reg := FrontEndRegistration{
			Name:    path.Base(frontEndNode.Key),
			Service: record.Service,
		}
		for _, sel := range record.Selectors {
			reg.Selectors = append(reg.Selectors, FrontEndSelector{
				Domain:     sel.Domain,
				PathPrefix: sel.PathPrefix,
			})
		}
		list = append(list, reg)
	}

	return list, nil
}
