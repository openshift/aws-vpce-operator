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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
)

func (c *AWSClient) GetDefaultPrivateHostedZoneId(domainName string) (*route53.HostedZone, error) {
	input := &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(domainName),
	}
	resp, err := c.Route53Client.ListHostedZonesByName(input)
	if err != nil {
		return nil, err
	}

	return resp.HostedZones[0], err
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
