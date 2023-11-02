package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
)

const (
	ttl    = 60
	weight = 100
)

func getDnsRecords(client *route53.Route53, zoneID string, zoneName string, svcName string) (IPs []string, records map[string]*route53.ResourceRecordSet, err error) {

	svcDns := svcName + "." + zoneName
	records = make(map[string]*route53.ResourceRecordSet)

	listParams := route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	}

	respList, err := client.ListResourceRecordSets(&listParams)
	if err != nil {
		return nil, nil, err
	}

	for _, record := range respList.ResourceRecordSets {
		if *record.Type == route53.RRTypeA && *record.Name == svcDns {
			ip := *record.ResourceRecords[0].Value
			IPs = append(IPs, ip)
			records[ip] = record
		}
	}

	return IPs, records, nil
}

func insertDnsRecord(client *route53.Route53, zoneID string, zoneName string, svcName string, ip string) (err error) {

	svcDns := svcName + "." + zoneName

	changeParams := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(svcDns),
						Type: aws.String("A"),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(ip),
							},
						},
						TTL:           aws.Int64(ttl),
						Weight:        aws.Int64(weight),
						SetIdentifier: aws.String(ip),
					},
				},
			},
		},
		HostedZoneId: aws.String(zoneID),
	}

	_, err = client.ChangeResourceRecordSets(changeParams)
	if err != nil {
		return err
	}

	return nil
}

func deleteDnsRecord(client *route53.Route53, zoneID string, zoneName string, record *route53.ResourceRecordSet) (err error) {

	changeParams := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String("DELETE"),
					ResourceRecordSet: record,
				},
			},
		},
		HostedZoneId: aws.String(zoneID),
	}

	_, err = client.ChangeResourceRecordSets(changeParams)
	if err != nil {
		return err
	}

	return nil
}
