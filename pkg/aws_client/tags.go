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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// CreateTags creates tags in an idempotent fashion
func (c *AWSClient) CreateTags(ctx context.Context, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return c.ec2Client.CreateTags(ctx, input)
}

// ListTagsForResource will fetch tags of a hosted zone or healthcheck
func (c *AWSClient) ListTagsForResource(ctx context.Context, params *route53.ListTagsForResourceInput) (*route53.ListTagsForResourceOutput, error) {
	return c.route53Client.ListTagsForResource(ctx, params)
}

// ChangeTagsForResource adds, edits, or deletes tags for a health check or a hosted zone.
func (c *AWSClient) ChangeTagsForResource(ctx context.Context, params *route53.ChangeTagsForResourceInput) (*route53.ChangeTagsForResourceOutput, error) {
	return c.route53Client.ChangeTagsForResource(ctx, params)
}
