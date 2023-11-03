package dns

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

type DNSManagerOption func(*DNSManagerOptions)

type DNSManagerOptions struct {
	ttl    int64
	weight int64
}

func WithTTL(ttl int64) DNSManagerOption {
	return func(dmo *DNSManagerOptions) {
		dmo.ttl = ttl
	}
}

func WithWeight(weight int64) DNSManagerOption {
	return func(dmo *DNSManagerOptions) {
		dmo.weight = weight
	}
}

type DNSManager struct {
	client   *route53.Route53
	options  *DNSManagerOptions
	zoneName *string
	zoneID   string
}

func NewDNSManager(zoneID string, options ...DNSManagerOption) (dm *DNSManager, err error) {
	// defaults options
	dmo := &DNSManagerOptions{
		ttl:    60,
		weight: 100,
	}

	for _, option := range options {
		option(dmo)
	}

	session, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	r53 := route53.New(session)

	zoneInput := route53.GetHostedZoneInput{
		Id: aws.String(zoneID),
	}
	zone, err := r53.GetHostedZone(&zoneInput)
	if err != nil {
		return nil, err
	}

	dm = &DNSManager{
		client:  r53,
		options: dmo,
		//serviceDNS: service + "." + *zone.HostedZone.Name,
		zoneName: zone.HostedZone.Name,
		zoneID:   zoneID,
	}

	return dm, nil
}

func (dm *DNSManager) GetDNSRecords(service string) (IPs []string, records map[string]*route53.ResourceRecordSet, err error) {
	serviceDNS := service + "." + *dm.zoneName
	records = make(map[string]*route53.ResourceRecordSet)

	listParams := route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(dm.zoneID),
	}

	respList, err := dm.client.ListResourceRecordSets(&listParams)
	if err != nil {
		return nil, nil, err
	}

	for _, record := range respList.ResourceRecordSets {
		if *record.Type == route53.RRTypeA && *record.Name == serviceDNS {
			ip := *record.ResourceRecords[0].Value
			IPs = append(IPs, ip)
			records[ip] = record
		}
	}

	return IPs, records, nil
}

func (dm *DNSManager) InsertDNSRecord(service string, ip string) (err error) {
	serviceDNS := service + "." + *dm.zoneName

	changeParams := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(serviceDNS),
						Type: aws.String("A"),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(ip),
							},
						},
						TTL:           aws.Int64(dm.options.ttl),
						Weight:        aws.Int64(dm.options.weight),
						SetIdentifier: aws.String(ip),
					},
				},
			},
		},
		HostedZoneId: aws.String(dm.zoneID),
	}

	_, err = dm.client.ChangeResourceRecordSets(changeParams)
	if err != nil {
		return err
	}

	return nil
}

func (dm *DNSManager) DeleteDNSRecord(record *route53.ResourceRecordSet) (err error) {
	changeParams := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String("DELETE"),
					ResourceRecordSet: record,
				},
			},
		},
		HostedZoneId: aws.String(dm.zoneID),
	}

	_, err = dm.client.ChangeResourceRecordSets(changeParams)
	if err != nil {
		return err
	}

	return nil
}
