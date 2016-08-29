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
	testService = Service{}
	configTests = []configTest{
		configTest{
			Service:    testService,
			Services:   backend.ServiceRegistrations{},
			ResultPath: "./fixtures/empty.txt",
		},
		configTest{
			Service: testService,
			Services: backend.ServiceRegistrations{
				backend.ServiceRegistration{
					ServiceName: "simple",
					ServicePort: 80,
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
