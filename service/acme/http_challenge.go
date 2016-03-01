// Copyright (c) 2016 Pulcy.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package acme

import (
	"fmt"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	"github.com/op/go-logging"
	"github.com/xenolf/lego/acme"
	"golang.org/x/net/context"
)

type HttpProviderConfig struct {
	EtcdPrefix string // Folder in ETCD to use for Http challenges
	Port       int    // Port to listen on
}

type HttpProviderDependencies struct {
	Logger     *logging.Logger
	EtcdClient client.Client
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
	kAPI := client.NewKeysAPI(s.EtcdClient)
	options := &client.SetOptions{
		TTL: 0,
	}
	if _, err := kAPI.Set(context.Background(), s.etcdTokenKey(token), keyAuth, options); err != nil {
		return maskAny(err)
	}
	return nil
}

func (s *httpChallengeProvider) CleanUp(domain, token, keyAuth string) error {
	// Remove token from etcdTokenKey
	kAPI := client.NewKeysAPI(s.EtcdClient)
	options := &client.DeleteOptions{
		Recursive: false,
	}
	if _, err := kAPI.Delete(context.Background(), s.etcdTokenKey(token), options); err != nil {
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
			kAPI := client.NewKeysAPI(s.EtcdClient)
			options := &client.GetOptions{
				Recursive: false,
				Sort:      false,
			}
			r, err := kAPI.Get(context.Background(), s.etcdTokenKey(token), options)
			if err != nil {
				s.Logger.Errorf("Failed to get keyAuth for token '%s'", token)
				// TODO
				return
			}
			keyAuth := r.Node.Value
			s.Logger.Debugf("Found keyAuth for token '%s'", token)
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
