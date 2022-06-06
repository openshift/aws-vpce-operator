/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws_client

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
)

func (c *AWSClient) GetDefaultPrivateHostedZoneId(domainName string) (*route53.HostedZone, error) {
	input := &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(domainName),
	}

	// TODO: Unlikely, but would be nice to handle pagination
	resp, err := c.Route53Client.ListHostedZonesByName(input)
	if err != nil {
		return nil, err
	}

	if len(resp.HostedZones) == 0 {
		return nil, fmt.Errorf("no hosted zone found for domain %s", domainName)
	}

	return resp.HostedZones[0], nil
}

func (c *AWSClient) ListResourceRecordSets(hostedZoneId string) (*route53.ListResourceRecordSetsOutput, error) {
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneId),
	}

	// TODO: Handle pagination
	resp, err := c.Route53Client.ListResourceRecordSets(input)
	if err != nil {
		return nil, err
	}

	return resp, err
}

// UpsertResourceRecordSet updates or creates a resource record set
func (c *AWSClient) UpsertResourceRecordSet(rrs *route53.ResourceRecordSet, hostedZoneId string) (*route53.ChangeResourceRecordSetsOutput, error) {
	input := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					// Upsert behavior: If a resource record set doesn't already exist, Route 53 creates it.
					// If a resource record set does exist, Route 53 updates it with the values in the request.
					Action:            aws.String("UPSERT"),
					ResourceRecordSet: rrs,
				},
			},
		},
		HostedZoneId: aws.String(hostedZoneId),
	}

	return c.Route53Client.ChangeResourceRecordSets(input)
}

// DeleteResourceRecordSet deletes a specific record from a hosted zone
// NOTE: To delete a resource record set, you must specify all the same values that you specified when you created it.
func (c *AWSClient) DeleteResourceRecordSet(rrs *route53.ResourceRecordSet, hostedZoneId string) (*route53.ChangeResourceRecordSetsOutput, error) {
	input := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String("DELETE"),
					ResourceRecordSet: rrs,
				},
			},
			Comment: aws.String(fmt.Sprintf("Deleting %s", *rrs.Name)),
		},
		HostedZoneId: aws.String(hostedZoneId),
	}

	return c.Route53Client.ChangeResourceRecordSets(input)
}
