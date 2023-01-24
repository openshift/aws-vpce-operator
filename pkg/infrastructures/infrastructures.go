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

package infrastructures

import (
	"context"
	"errors"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultInfrastructuresName = "cluster"

// GetAWSRegion returns the AWS region for the given cluster
func GetAWSRegion(ctx context.Context, c client.Client) (string, error) {
	infrastructures := new(configv1.Infrastructure)

	if err := c.Get(ctx, client.ObjectKey{Name: defaultInfrastructuresName}, infrastructures); err != nil {
		return "", fmt.Errorf("failed to get infrastructure %s: %w", defaultInfrastructuresName, err)
	}

	if infrastructures.Status.PlatformStatus.Type != "AWS" || infrastructures.Status.PlatformStatus.AWS == nil {
		return "", errors.New("platform is not AWS")
	}

	return infrastructures.Status.PlatformStatus.AWS.Region, nil
}

// GetInfrastructureName returns the .status.infrastructureName for the given cluster
func GetInfrastructureName(ctx context.Context, c client.Client) (string, error) {
	infrastructures := new(configv1.Infrastructure)

	if err := c.Get(ctx, client.ObjectKey{Name: defaultInfrastructuresName}, infrastructures); err != nil {
		return "", fmt.Errorf("failed to get infrastructure %s: %w", defaultInfrastructuresName, err)
	}

	return infrastructures.Status.InfrastructureName, nil
}
