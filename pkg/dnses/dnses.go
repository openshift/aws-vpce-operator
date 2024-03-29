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

package dnses

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DefaultDnsesName = "cluster"

// GetPrivateHostedZoneDomainName returns the domain name for the cluster's private hosted zone
func GetPrivateHostedZoneDomainName(ctx context.Context, c client.Client, name string) (string, error) {
	dnses := new(configv1.DNS)

	if err := c.Get(ctx, client.ObjectKey{Name: name}, dnses); err != nil {
		return "", err
	}

	return dnses.Spec.BaseDomain, nil
}
