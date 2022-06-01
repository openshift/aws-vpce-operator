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

package util

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	OperatorTagKey           = "kubernetes.io/private-service-operator"
	OperatorTagValue         = "managed"
	SecurityGroupDescription = "Managed by Private Service Operator"
)

func GenerateAwsTags(name, clusterTagKey string) ([]*ec2.Tag, error) {
	if name == "" {
		return nil, fmt.Errorf("name must be specified")
	}

	return []*ec2.Tag{
		{
			Key:   aws.String(OperatorTagKey),
			Value: aws.String(OperatorTagValue),
		},
		{
			Key:   aws.String(clusterTagKey),
			Value: aws.String("owned"),
		},
		{
			Key:   aws.String("Name"),
			Value: aws.String(name),
		},
	}, nil
}

// GetClusterTagKey returns the tag assigned to all AWS resources for the given cluster
func GetClusterTagKey(infraName string) (string, error) {
	if infraName == "" {
		return "", fmt.Errorf("infraName must be specified")
	}

	return fmt.Sprintf("kubernetes.io/cluster/%s", infraName), nil
}

// GenerateSecurityGroupName generates a name for a security group given a cluster name
// and a "purpose" for the security group
func GenerateSecurityGroupName(clusterName, purpose string) (string, error) {
	prefix := fmt.Sprintf("%s-%s", clusterName, purpose)
	return generateName(prefix, "sg", 255)
}

// GenerateVPCEndpointName generates a name for a VPC endpoint given a cluster name
// and a "purpose" for the VPC endpoint
func GenerateVPCEndpointName(clusterName, purpose string) (string, error) {
	prefix := fmt.Sprintf("%s-%s", clusterName, purpose)
	return generateName(prefix, "vpce", 255)
}

func generateName(prefix string, suffix string, maxLength int) (string, error) {
	if prefix == "" || suffix == "" {
		return "", fmt.Errorf("prefix and suffix must be specified")
	}

	if maxLength < 1 {
		return "", fmt.Errorf("maxLength must be greater than 0")
	}

	// Maximum length of a name is 255 characters
	if len(prefix) > (maxLength - len(suffix)) {
		prefix = prefix[:(maxLength - len(suffix))]
	}

	return fmt.Sprintf("%s-%s", prefix, suffix), nil
}
