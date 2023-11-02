package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/consul-server-connection-manager/discovery"
	capi "github.com/hashicorp/consul/api"
)

func getServiceEndpoints(client *capi.Client, service string) (endpoints []string, err error) {

	services, _, err := client.Health().Service(service, "", true, nil)
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

func newClient(address string) (client *capi.Client, err error) {

	config := capi.DefaultConfig()
	config.Address = address
	config.HttpClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	client, err = capi.NewClient(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func newClientFromConnMgr(watcher *discovery.Watcher) (client *capi.Client, err error) {

	state, err := watcher.State()
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("%s:%d", state.Address.IP, 8500)

	client, err = newClient(address)
	if err != nil {
		return nil, err
	}

	return client, nil
}
