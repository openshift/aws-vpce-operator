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

package secrets

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultAWSAccessKeyId     = "aws_access_key_id"     //#nosec G101
	defaultAWSSecretAccessKey = "aws_secret_access_key" //#nosec G101
)

// ParseAWSCredentialOverride takes in an AWS region and a secret reference and attempts to assemble an aws.Config
// Currently only supports parsing AWS IAM User credentials
func ParseAWSCredentialOverride(ctx context.Context, c client.Client, region string, ref *corev1.SecretReference) (aws.Config, error) {
	if ref == nil {
		return aws.Config{}, errors.New("AWS Credential Override secret reference must not be nil")
	}

	secret := new(corev1.Secret)
	if err := c.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return aws.Config{}, err
	}

	if b64AccessKeyId, ok := secret.Data[defaultAWSAccessKeyId]; ok {
		if b64SecretAccessKey, ok := secret.Data[defaultAWSSecretAccessKey]; ok {
			accessKeyId, err := base64.StdEncoding.DecodeString(string(b64AccessKeyId))
			if err != nil {
				return aws.Config{}, err
			}

			secretAccessKey, err := base64.StdEncoding.DecodeString(string(b64SecretAccessKey))
			if err != nil {
				return aws.Config{}, err
			}

			return config.LoadDefaultConfig(ctx,
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(string(accessKeyId), string(secretAccessKey), "")),
				config.WithRegion(region),
			)
		}
	}

	return aws.Config{}, fmt.Errorf("could not parse credential override secret, requires data keys %s and %s", defaultAWSAccessKeyId, defaultAWSSecretAccessKey)
}
