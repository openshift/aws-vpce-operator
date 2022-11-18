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
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/openshift/aws-vpce-operator/pkg/util"
)

// GetDefaultPrivateHostedZoneId returns the cluster's Route53 private hosted zone
func (c *AWSClient) GetDefaultPrivateHostedZoneId(ctx context.Context, domainName string) (*types.HostedZone, error) {
	input := &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(domainName),
	}

	// TODO: Unlikely, but would be nice to handle pagination
	resp, err := c.route53Client.ListHostedZonesByName(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(resp.HostedZones) == 0 {
		return nil, fmt.Errorf("no hosted zone found for domain %s", domainName)
	}

	return &resp.HostedZones[0], nil
}

// ListResourceRecordSets returns a list of records for a given hosted zone ID
func (c *AWSClient) ListResourceRecordSets(ctx context.Context, hostedZoneId string) (*route53.ListResourceRecordSetsOutput, error) {
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneId),
	}

	// TODO: Handle pagination
	resp, err := c.route53Client.ListResourceRecordSets(ctx, input)
	if err != nil {
		return nil, err
	}

	return resp, err
}

// UpsertResourceRecordSet updates or creates a resource record set
func (c *AWSClient) UpsertResourceRecordSet(ctx context.Context, rrs *types.ResourceRecordSet, hostedZoneId string) (*route53.ChangeResourceRecordSetsOutput, error) {
	input := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					// Upsert behavior: If a resource record set doesn't already exist, Route 53 creates it.
					// If a resource record set does exist, Route 53 updates it with the values in the request.
					Action:            types.ChangeActionUpsert,
					ResourceRecordSet: rrs,
				},
			},
		},
		HostedZoneId: aws.String(hostedZoneId),
	}

	return c.route53Client.ChangeResourceRecordSets(ctx, input)
}

// DeleteResourceRecordSet deletes a specific record from a hosted zone
// NOTE: To delete a resource record set, you must specify all the same values that you specified when you created it.
func (c *AWSClient) DeleteResourceRecordSet(ctx context.Context, rrs *types.ResourceRecordSet, hostedZoneId string) (*route53.ChangeResourceRecordSetsOutput, error) {
	input := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action:            types.ChangeActionDelete,
					ResourceRecordSet: rrs,
				},
			},
			Comment: aws.String(fmt.Sprintf("Deleting %s", *rrs.Name)),
		},
		HostedZoneId: aws.String(hostedZoneId),
	}

	return c.route53Client.ChangeResourceRecordSets(ctx, input)
}

// CreateNewHostedZone is an implementation of the client's CreateHostedZone
// NOTE: To associate additional Amazon VPCs with the hosted zone, use AssociateVPCWithHostedZone after you create a hosted zone.
func (c *AWSClient) CreateNewHostedZone(ctx context.Context, domain, vpcId, callerRef, region string) (*route53.CreateHostedZoneOutput, error) {
	zoneInput := &route53.CreateHostedZoneInput{
		CallerReference:  aws.String(callerRef),
		Name:             aws.String(domain),
		HostedZoneConfig: &types.HostedZoneConfig{Comment: aws.String("Created via AVO"), PrivateZone: true},
		VPC:              &types.VPC{VPCId: aws.String(vpcId), VPCRegion: types.VPCRegion(region)},
	}
	return c.route53Client.CreateHostedZone(ctx, zoneInput)
}

// GenerateDefaultTagsForHostedZoneInput generates the ChangeTagsForResourceInput using the default tags for the zoneId
func (c *AWSClient) GenerateDefaultTagsForHostedZoneInput(zoneId, clusterTagKey string) (*route53.ChangeTagsForResourceInput, error) {
	defaultTags, err := util.GenerateR53Tags(clusterTagKey)

	changeTagsInput := &route53.ChangeTagsForResourceInput{
		ResourceId:    aws.String(zoneId),
		ResourceType:  types.TagResourceTypeHostedzone,
		AddTags:       defaultTags,
		RemoveTagKeys: nil,
	}
	return changeTagsInput, err
}

// FetchPrivateZoneTags takes context and a Route53 ZoneID and returns the output provided by ListTagsForResource for a hosted zone
func (c *AWSClient) FetchPrivateZoneTags(ctx context.Context, zoneId string) (*route53.ListTagsForResourceOutput, error) {
	return c.route53Client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceId:   aws.String(zoneId),
		ResourceType: types.TagResourceTypeHostedzone,
	})
}
