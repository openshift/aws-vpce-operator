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

package vpcendpoint

import "github.com/openshift/aws-vpce-operator/api/v1alpha1"

// deleteAWSResources cleans up AWS resources associated with a VPC Endpoint.
func (r *VpcEndpointReconciler) deleteAWSResources(resource *v1alpha1.VpcEndpoint) error {
	if resource.Status.VPCEndpointId != "" {
		r.Log.V(0).Info("Deleting AWS resources", "VpcEndpoint", resource.Status.VPCEndpointId)
		if _, err := r.AWSClient.DeleteVPCEndpoint(resource.Status.VPCEndpointId); err != nil {
			return err
		}
	}

	if resource.Status.SecurityGroupId != "" {
		r.Log.V(0).Info("Deleting AWS resources", "SecurityGroup", resource.Status.SecurityGroupId)
		if _, err := r.AWSClient.DeleteSecurityGroup(resource.Status.SecurityGroupId); err != nil {
			return err
		}
	}

	return nil
}
