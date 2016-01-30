package acme

import (
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/op/go-logging"
	"github.com/xenolf/lego/acme"
)

type HttpProviderConfig struct {
	EtcdPrefix string // Folder in ETCD to use for Http challenges
	Port       int    // Port to listen on
}

type HttpProviderDependencies struct {
	Logger     *logging.Logger
	EtcdClient *etcd.Client
}

type httpChallengeProvider struct {
	HttpProviderConfig
	HttpProviderDependencies
}

func newHttpChallengeProvider(config HttpProviderConfig, deps HttpProviderDependencies) *httpChallengeProvider {
	return &httpChallengeProvider{
		HttpProviderConfig:       config,
		HttpProviderDependencies: deps,
	}
}

// Present makes the token available at `HTTP01ChallengePath(token)`
func (s *httpChallengeProvider) Present(domain, token, keyAuth string) error {
	// Write token & keyAuth in ETCD
	if _, err := s.EtcdClient.Set(s.etcdTokenKey(token), keyAuth, 0); err != nil {
		return maskAny(err)
	}
	return nil
}

func (s *httpChallengeProvider) CleanUp(domain, token, keyAuth string) error {
	// Remove token from etcdTokenKey
	if _, err := s.EtcdClient.Delete(s.etcdTokenKey(token), false); err != nil {
		return maskAny(err)
	}
	return nil
}

// Start launches an HTTP service that handles HTTP challenge requests
func (s *httpChallengeProvider) Start() error {
	pathPrefix := acme.HTTP01ChallengePath("")

	var handler http.HandlerFunc
	handler = func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if strings.HasPrefix(path, pathPrefix) && req.Method == "GET" {
			token := path[len(pathPrefix):]
			r, err := s.EtcdClient.Get(s.etcdTokenKey(token), false, false)
			if err != nil {
				s.Logger.Error("Failed to get keyAuth for token '%s'", token)
				// TODO
				return
			}
			keyAuth := r.Node.Value
			s.Logger.Debug("Found keyAuth for token '%s'", token)
			w.Header().Add("Content-Type", "text/plain")
			w.Write([]byte(keyAuth))
		} else {
			s.Logger.Warning("Unknown token request: %s", path)
			w.Write([]byte("HELLO"))
		}
	}

	listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(s.Port)))
	if err != nil {
		return maskAny(fmt.Errorf("Could not start HTTP server for challenge: %#v", err))
	}

	go http.Serve(listener, handler)
	return nil
}

func (s *httpChallengeProvider) etcdTokenKey(token string) string {
	return path.Join(s.EtcdPrefix, token)
}
