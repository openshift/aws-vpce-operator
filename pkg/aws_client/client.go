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
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
)

type AWSClient struct {
	ec2Client     ec2iface.EC2API
	route53Client route53iface.Route53API
}

// NewAwsClient returns an AWSClient with the provided session
func NewAwsClient(sess *session.Session) *AWSClient {
	return NewAwsClientWithServiceClients(ec2.New(sess), route53.New(sess))
}

// NewAwsClientWithServiceClients returns an AWSClient with the provided EC2 and Route53 clients.
// Typically, not used directly except for building a mock for testing.
func NewAwsClientWithServiceClients(ec2 ec2iface.EC2API, r53 route53iface.Route53API) *AWSClient {
	return &AWSClient{
		ec2Client:     ec2,
		route53Client: r53,
	}
}
