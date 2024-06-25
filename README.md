consul to route53 syncer. usefull for publishing consul services on public dns.

```
Usage of ./syncer:
  -consul-address="127.0.0.1": address list of consul server
  -consul-grpc-port=8502: grpc port of consul server
  -consul-http-port=8500: http port of consul server
  -consul-http-timeout=5: http timeout for connecting to consul server
  -consul-services="": comma separated consul services to synchronize [REQUIRED]
  -dns-zone-ids="": comma separated route53 zone-ID to synchronize to [REQUIRED]
  -refresh-interval=20: interval between sync
```

