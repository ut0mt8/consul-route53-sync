package main

import (
	"slices"
	"strings"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/namsral/flag"

	"consul-route53-sync/internal/consul"
	"consul-route53-sync/internal/dns"
)

type config struct {
	addresses string
	grpc      int
	http      int
	timeout   int
	zoneID    string
	services  string
	interval  int
}

func main() {

	var conf config

	log := hclog.New(&hclog.LoggerOptions{
		Name: "syncer",
	})

	flag.StringVar(&conf.addresses, "consul-addresses", "", "go-netaddrs formated consul servers defintion [REQUIRED]")
	flag.IntVar(&conf.grpc, "consul-grpc-port", 8502, "grpc port of consul server")
	flag.IntVar(&conf.http, "consul-http-port", 8500, "http port of consul server")
	flag.IntVar(&conf.timeout, "consul-http-timeout", 5, "http timeout for connecting to consul server")
	flag.StringVar(&conf.services, "consul-services", "", "comma separated consul services to synchronize [REQUIRED]")
	flag.StringVar(&conf.zoneID, "dns-zone-id", "", "route53 zone-ID to synchronize to [REQUIRED]")
	flag.IntVar(&conf.interval, "refresh-interval", 20, "interval between sync")
	flag.Parse()

	if conf.addresses == "" && conf.services == "" || conf.zoneID == "" {
		flag.Usage()
		log.Error("required parameters missing")
		return
	}

	services := strings.Split(conf.services, ",")

	cm, err := consul.NewConsulManager(
		conf.addresses,
		consul.WithGRPCPort(conf.grpc),
		consul.WithHTTPPort(conf.http),
		consul.WithTimeout(conf.timeout),
	)
	if err != nil {
		log.Error("create consul manager", "error", hclog.Fmt("%s", err))
		return
	}

	go cm.Run()
	defer cm.Stop()

	dm, err := dns.NewDNSManager(
		conf.zoneID,
	)
	if err != nil {
		log.Error("create dns manager", "error", hclog.Fmt("%s", err))
		return
	}

	for range time.NewTicker(time.Duration(conf.interval) * time.Second).C {

		for _, service := range services {

			log.Info("sync fired", "service", service)

			endpoints, err := cm.GetServiceEndpoints(service)
			if err != nil {
				log.Error("consul get endpoints", "service", service, "error", hclog.Fmt("%s", err))
				continue
			}

			entries, records, err := dm.GetDNSRecords(service)
			if err != nil {
				log.Error("get dns records", "service", service, "error", hclog.Fmt("%s", err))
				continue
			}

			// ensure all consul endpoints are in dns
			for _, endpoint := range endpoints {
				if slices.Contains(entries, endpoint) {
					log.Debug("add-to-dns", "service", service, "existing record", hclog.Fmt("%s", endpoint), "action", "none")
				} else {
					log.Info("add-to-dns", "service", service, "non existing record", hclog.Fmt("%s", endpoint), "action", "add")
					err := dm.InsertDNSRecord(service, endpoint)
					if err != nil {
						log.Error("insert dns record", "service", service, "error", hclog.Fmt("%s", err))
						continue
					}
				}
			}

			// deleting stale dns records
			for _, entry := range entries {
				if slices.Contains(endpoints, entry) {
					log.Debug("clean-dns", "service", service, "existing endpoint", hclog.Fmt("%s", entry), "action", "none")
				} else {
					log.Info("clean-dns", "service", service, "non existing endpoint", hclog.Fmt("%s", entry), "action", "delete")
					err := dm.DeleteDNSRecord(records[entry])
					if err != nil {
						log.Error("delete dns record", "service", service, "error", hclog.Fmt("%s", err))
						continue
					}
				}
			}
		}
	}
}
