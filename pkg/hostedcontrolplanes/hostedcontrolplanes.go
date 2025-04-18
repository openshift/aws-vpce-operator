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

package hostedcontrolplanes

import (
	"context"
	"fmt"

	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetInfraId returns the infra id of a hostedcontrolplane
func GetInfraId(ctx context.Context, c client.Client, namespace string) (string, error) {
	hcpList := new(hyperv1beta1.HostedControlPlaneList)

	if err := c.List(ctx, hcpList, &client.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return "", err
	}

	if len(hcpList.Items) == 1 {
		if hcpList.Items[0].Spec.InfraID == "" {
			return "", fmt.Errorf("blank .spec.infraId for %s", hcpList.Items[0].Name)
		}

		return hcpList.Items[0].Spec.InfraID, nil
	}

	return "", fmt.Errorf("found %d hostedcontrolplanes in namespace: %s, expected 1", len(hcpList.Items), namespace)
}

// GetPrivateHostedZoneDomainName returns the domain name for a hosted cluster's private hosted zone
func GetPrivateHostedZoneDomainName(ctx context.Context, c client.Client, namespace string) (string, error) {
	hcpList := new(hyperv1beta1.HostedControlPlaneList)

	if err := c.List(ctx, hcpList, &client.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return "", err
	}

	if len(hcpList.Items) == 1 {
		if hcpList.Items[0].Spec.Services != nil {
			for _, svc := range hcpList.Items[0].Spec.Services {
				if svc.Service == hyperv1beta1.APIServer {
					if svc.Type == hyperv1beta1.Route && svc.Route.Hostname != "" {
						// The hostname contains the full api.${basedomain}, so take out the leading "api"
						var domainName string
						if _, err := fmt.Sscanf(svc.Route.Hostname, "api.%s", &domainName); err != nil {
							return "", err
						}
						return domainName, nil
					} else {
						return "", fmt.Errorf("unable to find APIServer route hostname in hostedcontrolplane .spec.services")
					}
				}
			}
		}

		return "", fmt.Errorf("unable to find APIServer url in hostedcontrolplane .spec.services")
	}

	return "", fmt.Errorf("found %d hostedcontrolplanes in namespace: %s, expected 1", len(hcpList.Items), namespace)
}
