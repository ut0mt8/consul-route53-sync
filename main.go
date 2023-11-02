package main

import (
	"context"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/hashicorp/consul-server-connection-manager/discovery"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/namsral/flag"
)

type config struct {
	address  string
	zoneID   string
	service  string
	interval int
}

func main() {

	var conf config

	log := hclog.New(&hclog.LoggerOptions{
		Name: "syncer",
	})

	flag.StringVar(&conf.address, "consul-address", "127.0.0.1", "address list of consul server to connect to")
	flag.StringVar(&conf.zoneID, "zone-id", "", "route53 zone-ID to synchronize to")
	flag.StringVar(&conf.service, "service", "", "consul service to synchronize")
	flag.IntVar(&conf.interval, "refresh-interval", 10, "interval between sync")
	flag.Parse()

	if conf.zoneID == "" || conf.service == "" {
		flag.Usage()
		log.Error("required parameters missing")
		return
	}

	watcher, err := discovery.NewWatcher(
		context.Background(),
		discovery.Config{
			Addresses: conf.address,
			GRPCPort:  8502,
		},
		hclog.New(&hclog.LoggerOptions{
			Name: "consul-watcher",
		}),
	)
	if err != nil {
		log.Error("create consul watcher", "error", hclog.Fmt("%s", err))
		return
	}

	go watcher.Run()
	defer watcher.Stop()

	for t := range time.NewTicker(time.Duration(conf.interval) * time.Second).C {

		log.Info("sync fired", "time", hclog.Fmt("%v", t))

		c, err := newClientFromConnMgr(watcher)
		if err != nil {
			log.Error("create consul client", "error", hclog.Fmt("%s", err))
			continue
		}

		endpoints, err := getServiceEndpoints(c, conf.service)
		if err != nil {
			log.Error("consul get services", "error", hclog.Fmt("%s", err))
			continue
		}

		session, err := session.NewSession()
		if err != nil {
			log.Error("aws create session", "error", hclog.Fmt("%s", err))
			continue
		}

		r53 := route53.New(session)

		zoneInput := route53.GetHostedZoneInput{
			Id: &conf.zoneID,
		}
		zone, err := r53.GetHostedZone(&zoneInput)
		if err != nil {
			log.Error("get hosted zone", "error", hclog.Fmt("%s", err))
			continue
		}

		entries, records, err := getDnsRecords(r53, conf.zoneID, *zone.HostedZone.Name, conf.service)
		if err != nil {
			log.Error("get dns record", "error", hclog.Fmt("%s", err))
			continue
		}

		// ensure all consul endpoints are in dns
		for _, endpoint := range endpoints {
			if slices.Contains(entries, endpoint) {
				log.Debug("add-to-route53", "existing record", hclog.Fmt("%s", endpoint), "action", "none")
			} else {
				log.Info("add-to-route53", "non existing record", hclog.Fmt("%s", endpoint), "action", "add")
				err := insertDnsRecord(r53, conf.zoneID, *zone.HostedZone.Name, conf.service, endpoint)
				if err != nil {
					log.Error("insert dns record", "error", hclog.Fmt("%s", err))
					continue
				}
			}
		}

		// deleting stale dns records
		for _, entry := range entries {
			if slices.Contains(endpoints, entry) {
				log.Debug("clean-route53", "existing endpoint", hclog.Fmt("%s", entry), "action", "none")
			} else {
				log.Info("clean-route53", "non existing endpoint", hclog.Fmt("%s", entry), "action", "delete")
				err := deleteDnsRecord(r53, "Z01631192GAC9VANOIKPQ", *zone.HostedZone.Name, records[entry])
				if err != nil {
					log.Error("delete dns record", "error", hclog.Fmt("%s", err))
					continue
				}
			}
		}
	}

}
