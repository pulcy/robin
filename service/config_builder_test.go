package service

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/pulcy/robin/service/backend"
)

type configTest struct {
	Service    Service
	Services   backend.ServiceRegistrations
	ResultPath string
}

var (
	testService         = Service{}
	privateStatsService = Service{
		ServiceConfig: ServiceConfig{
			PrivateStatsPort: 1234,
		},
	}
	configTests = []configTest{
		configTest{
			Service:    testService,
			Services:   backend.ServiceRegistrations{},
			ResultPath: "./fixtures/empty.txt",
		},
		configTest{
			Service:    privateStatsService,
			Services:   backend.ServiceRegistrations{},
			ResultPath: "./fixtures/empty-private-stats.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "simple",
					ServicePort: 80,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					Mode: "http",
				},
			},
			ResultPath: "./fixtures/simple_service_2_instances.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "simple12",
					ServicePort: 80,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					Mode: "http",
				},
				backend.ServiceRegistration{
					ServiceName: "simple2",
					ServicePort: 5000,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.3", Port: 7001},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo2.com",
						},
					},
					Mode: "http",
				},
				backend.ServiceRegistration{
					ServiceName: "simple3",
					ServicePort: 5000,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.3", Port: 7001},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							PathPrefix: "/prefix",
							Weight:     100,
						},
					},
					Mode: "http",
				},
			},
			ResultPath: "./fixtures/simple_services.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "service1",
					ServicePort: 80,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					Mode: "http",
				},
				backend.ServiceRegistration{
					ServiceName: "service2",
					ServicePort: 5000,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.3", Port: 7001},
						backend.ServiceInstance{IP: "192.168.23.1", Port: 7005},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					Mode:            "http",
					HttpCheckMethod: "OPTIONS",
				},
				backend.ServiceRegistration{
					ServiceName: "service3_prefix",
					ServicePort: 4700,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.3", Port: 7001},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain:     "foo.com",
							PathPrefix: "/prefix",
						},
					},
					Mode: "http",
				},
				backend.ServiceRegistration{
					ServiceName: "service4_small_prefix_only",
					ServicePort: 4700,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.3", Port: 9001},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							PathPrefix: "/prefix-only",
						},
					},
					Mode: "http",
				},
				backend.ServiceRegistration{
					ServiceName: "service4_large_prefix_only",
					ServicePort: 6004,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.33", Port: 19001},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							PathPrefix: "/prefix/large",
						},
					},
					Mode: "http",
				},
			},
			ResultPath: "./fixtures/same_domain_services.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "master",
					ServicePort: 80,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					HttpCheckMethod: "GET",
					Mode:            "http",
				},
				backend.ServiceRegistration{
					ServiceName: "slave",
					ServicePort: 5000,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.35", Port: 7001},
						backend.ServiceInstance{IP: "192.168.35.37", Port: 7007},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					Mode:   "http",
					Backup: true,
				},
			},
			ResultPath: "./fixtures/backup_services.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "private1",
					ServicePort: 80,
					Public:      false,
					EdgePort:    PrivateHttpPort,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "service1.private",
						},
					},
					Mode: "http",
				},
			},
			ResultPath: "./fixtures/private_service.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "service1",
					ServicePort: 80,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
						backend.ServiceInstance{IP: "192.168.23.32", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
						backend.ServiceSelector{
							Domain:     "nested.foo.com",
							PathPrefix: "/foo",
						},
					},
					Mode: "http",
				},
				backend.ServiceRegistration{
					ServiceName: "service1",
					ServicePort: 80,
					EdgePort:    PrivateHttpPort,
					Public:      false,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
						backend.ServiceInstance{IP: "192.168.23.32", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com.private",
						},
						backend.ServiceSelector{
							Domain: "service1.private",
						},
					},
					Mode: "http",
				},
			},
			ResultPath: "./fixtures/many_frontends_service.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "sticky1",
					ServicePort: 80,
					EdgePort:    PublicHttpPort,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
						backend.ServiceInstance{IP: "192.168.23.32", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{
							Domain: "foo.com",
						},
					},
					Sticky: true,
					Mode:   "http",
				},
			},
			ResultPath: "./fixtures/sticky_service.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "gogs",
					ServicePort: 22,
					EdgePort:    8022,
					Public:      true,
					Instances: backend.ServiceInstances{
						backend.ServiceInstance{IP: "192.168.35.2", Port: 2345},
						backend.ServiceInstance{IP: "192.168.35.3", Port: 2346},
						backend.ServiceInstance{IP: "192.168.23.32", Port: 2346},
					},
					Selectors: backend.ServiceSelectors{
						backend.ServiceSelector{},
					},
					Sticky: true,
					Mode:   "tcp",
				},
			},
			ResultPath: "./fixtures/ssh_gogs.txt",
		},
	}
)

func TestConfigs(t *testing.T) {
	updateFixtures := os.Getenv("UPDATE-FIXTURES") == "1"
	for _, test := range configTests {
		result, err := test.Service.renderConfig(test.Services)
		if err != nil {
			t.Errorf("Test failed: %#v", err)
		} else {
			if updateFixtures {
				err := ioutil.WriteFile(test.ResultPath, []byte(result), 0644)
				if err != nil {
					t.Errorf("Cannot update fixture %s: %#v", test.ResultPath, err)
				}
			} else {
				expectedRaw, err := ioutil.ReadFile(test.ResultPath)
				if err != nil {
					t.Errorf("Cannot read fixture %s: %#v", test.ResultPath, err)
				} else {
					expected := strings.Split(string(expectedRaw), "\n")
					lines := strings.Split(result, "\n")
					for i, line := range lines {
						if i >= len(expected) {
							t.Errorf("Unexpected addition: `%s`", line)
							break
						} else if expected[i] != line {
							t.Errorf("Diff at %d: expected `%s` got `%s`", i, expected[i], line)
							break
						}
					}
				}
			}
		}
	}
}
