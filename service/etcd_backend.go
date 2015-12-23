package service

import (
	"encoding/json"
	"net/url"
	"path"
	"strconv"
	"strings"

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
		name := path.Base(serviceNode.Key)
		registrations := make(map[int]*ServiceRegistration)
		for _, backendNode := range serviceNode.Nodes {
			uniqueID := path.Base(backendNode.Key)
			parts := strings.Split(uniqueID, ":")
			if len(parts) < 3 {
				eb.Logger.Warning("UniqueID malformed: '%s'", uniqueID)
				continue
			}
			port, err := strconv.Atoi(parts[2])
			if err != nil {
				eb.Logger.Warning("Failed to parse port: '%s'", parts[2])
				continue
			}
			sr, ok := registrations[port]
			if !ok {
				sr = &ServiceRegistration{ServiceName: name, Port: port}
				registrations[port] = sr
			}
			sr.Backends = append(sr.Backends, backendNode.Value)
		}
		for _, v := range registrations {
			list = append(list, *v)
		}
	}

	return list, nil
}

type frontendRecord struct {
	Selectors     []frontendSelectorRecord `json:"selectors"`
	Service       string                   `json:"service,omitempty"`
	HttpCheckPath string                   `json:"http-check-path,omitempty"`
}

type frontendSelectorRecord struct {
	Domain     string       `json:"domain,omitempty"`
	SslCert    string       `json:"ssl-cert,omitempty"`
	PathPrefix string       `json:"path-prefix,omitempty"`
	Port       int          `json:"port,omitempty"`
	Private    bool         `json:"private,omitempty"`
	Users      []userRecord `json:"users,omitempty"`
}

type userRecord struct {
	Name         string `json:"user"`
	PasswordHash string `json:"pwhash"`
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

		name := path.Base(frontEndNode.Key)
		registrations := make(map[int]*FrontEndRegistration)
		for _, sel := range record.Selectors {
			port := sel.Port
			reg, ok := registrations[port]
			if !ok {
				reg = &FrontEndRegistration{
					Name:          name,
					Service:       record.Service,
					Port:          port,
					HttpCheckPath: record.HttpCheckPath,
				}
				registrations[port] = reg
			}
			frSel := FrontEndSelector{
				Domain:     sel.Domain,
				SslCert:    sel.SslCert,
				PathPrefix: sel.PathPrefix,
				Port:       sel.Port,
				Private:    sel.Private,
			}
			for _, user := range sel.Users {
				frSel.Users = append(frSel.Users, User{
					Name:         user.Name,
					PasswordHash: user.PasswordHash,
				})
			}
			reg.Selectors = append(reg.Selectors, frSel)
		}
		for _, reg := range registrations {
			list = append(list, *reg)
		}
	}

	return list, nil
}
