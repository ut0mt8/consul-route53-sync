package consul

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/consul-server-connection-manager/discovery"
	capi "github.com/hashicorp/consul/api"
	hclog "github.com/hashicorp/go-hclog"
)

type ConsulManagerOption func(*ConsulManagerOptions)

type ConsulManagerOptions struct {
	grpcPort int
	httpPort int
	timeout  int
}

func WithGRPCPort(port int) ConsulManagerOption {
	return func(cmo *ConsulManagerOptions) {
		cmo.grpcPort = port
	}
}

func WithHTTPPort(port int) ConsulManagerOption {
	return func(cmo *ConsulManagerOptions) {
		cmo.httpPort = port
	}
}

func WithTimeout(timeout int) ConsulManagerOption {
	return func(cmo *ConsulManagerOptions) {
		cmo.timeout = timeout
	}
}

type ConsulManager struct {
	client  *capi.Client
	options *ConsulManagerOptions
	watcher *discovery.Watcher
}

func NewConsulManager(address string, options ...ConsulManagerOption) (cm *ConsulManager, err error) {
	// defaults options
	cmo := &ConsulManagerOptions{
		grpcPort: 8502,
		httpPort: 8500,
		timeout:  5,
	}

	for _, option := range options {
		option(cmo)
	}

	watcher, err := discovery.NewWatcher(
		context.Background(),
		discovery.Config{
			Addresses: address,
			GRPCPort:  cmo.grpcPort,
		},
		hclog.New(&hclog.LoggerOptions{
			Name: "consul-watcher",
		}),
	)
	if err != nil {
		return nil, err
	}

	cm = &ConsulManager{
		options: cmo,
		watcher: watcher,
	}

	return cm, nil
}

func (cm *ConsulManager) Run() {
	cm.watcher.Run()
}

func (cm *ConsulManager) Stop() {
	cm.watcher.Stop()
}

func (cm *ConsulManager) renewClient() (err error) {
	state, err := cm.watcher.State()
	if err != nil {
		return err
	}

	consulConfig := capi.DefaultConfig()
	consulConfig.Address = fmt.Sprintf("%s:%d", state.Address.IP, cm.options.httpPort)
	consulConfig.HttpClient = &http.Client{
		Timeout: time.Duration(cm.options.timeout) * time.Second,
	}

	client, err := capi.NewClient(consulConfig)
	if err != nil {
		return err
	}

	cm.client = client

	return nil
}

func (cm *ConsulManager) GetServiceEndpoints(service string) (endpoints []string, err error) {
	err = cm.renewClient()
	if err != nil {
		return nil, err
	}

	services, _, err := cm.client.Health().Service(service, "", true, nil)
	if len(services) == 0 && err == nil {
		return nil, fmt.Errorf("service not found")
	}
	if err != nil {
		return nil, err
	}

	for _, svc := range services {
		if svc.Service.Address == "" {
			endpoints = append(endpoints, svc.Node.Address)
		} else {
			endpoints = append(endpoints, svc.Service.Address)
		}
	}

	return endpoints, nil
}
