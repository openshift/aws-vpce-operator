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

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/dnses"
	"github.com/openshift/aws-vpce-operator/pkg/infrastructures"
	"github.com/openshift/aws-vpce-operator/pkg/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// defaultAVOLogger returns a zap.Logger using RFC3339 timestamps for the vpcendpoint controller
func defaultAVOLogger() (logr.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	// TODO: Make this configurable
	// config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)

	zapBase, err := config.Build()
	if err != nil {
		return logr.Logger{}, err
	}

	logger := zapr.NewLogger(zapBase)
	return logger.WithName(controllerName), nil
}

// defaultAVORateLimiter returns a rate limiter that reconciles more slowly than the default.
// The default is 5ms --> 1000s, but resources are created much more slowly in AWS than in
// Kubernetes, so this helps avoid AWS rate limits.
// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/throttling.html#throttling-limits
func defaultAVORateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 5000*time.Second),
		// 10 qps, 100 bucket size, only for overall retry limiting (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(10, 100)},
	)
}

// parseClusterInfo fills in the clusterInfo struct values inside the VpcEndpointReconciler
// and gets a new AWS session if refreshAWSSession is true.
// Generally, refreshAWSSession is only set to false during testing to mock the AWS client.
func (r *VpcEndpointReconciler) parseClusterInfo(ctx context.Context, refreshAWSSession bool) error {
	r.clusterInfo = new(clusterInfo)

	region, err := infrastructures.GetAWSRegion(ctx, r.Client)
	if err != nil {
		return err
	}
	r.clusterInfo.region = region
	r.log.V(1).Info("Parsed region from infrastructure", "region", region)

	if refreshAWSSession {
		sess, err := session.NewSession(&aws.Config{
			Region: &region,
		})
		if err != nil {
			return err
		}
		r.awsClient = aws_client.NewAwsClient(sess)
	}

	infraName, err := infrastructures.GetInfrastructureName(ctx, r.Client)
	if err != nil {
		return err
	}
	r.clusterInfo.infraName = infraName
	r.log.V(1).Info("Found infrastructure name:", "name", infraName)

	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return err
	}
	r.clusterInfo.clusterTag = clusterTag
	r.log.V(1).Info("Found cluster tag:", "clusterTag", clusterTag)

	vpcId, err := r.awsClient.GetVPCId(r.clusterInfo.clusterTag)
	if err != nil {
		return err
	}
	r.clusterInfo.vpcId = vpcId
	r.log.V(1).Info("Found vpc id:", "vpcId", vpcId)

	domainName, err := dnses.GetPrivateHostedZoneDomainName(ctx, r.Client)
	if err != nil {
		return err
	}
	r.clusterInfo.domainName = domainName
	r.log.V(1).Info("Found domain name:", "domainName", domainName)

	return nil
}

// findOrCreateSecurityGroup queries AWS and returns the Security Group for the provided CR and updates its status.
// It first tries to use the Security Group ID that may be in the resource's status and falls back on
// searching for the VPC Endpoint by tags in case the status is lost. If it still cannot find a Security Group,
// it gets created.
func (r *VpcEndpointReconciler) findOrCreateSecurityGroup(ctx context.Context, resource *avov1alpha1.VpcEndpoint) (*ec2.SecurityGroup, error) {
	var sg *ec2.SecurityGroup

	r.log.V(1).Info("Searching for security group by ID", "id", resource.Status.SecurityGroupId)
	resp, err := r.awsClient.FilterSecurityGroupById(resource.Status.SecurityGroupId)
	if err != nil {
		return nil, err
	}

	// If there's no security group returned by ID, look for one by tag
	// first, generate the security group name to search tags or use it later to create it
	if resp == nil || len(resp.SecurityGroups) == 0 {
		sgName, err := util.GenerateSecurityGroupName(r.clusterInfo.infraName, resource.Name)
		if err != nil {
			return nil, err
		}

		r.log.V(1).Info("Searching for security group by tags")
		resp, err = r.awsClient.FilterSecurityGroupByDefaultTags(r.clusterInfo.infraName, sgName)
		if err != nil {
			return nil, err
		}

		// If there are still no security groups found, it needs to be created
		if resp == nil || len(resp.SecurityGroups) == 0 {

			createResp, err := r.awsClient.CreateSecurityGroup(sgName, r.clusterInfo.vpcId, r.clusterInfo.clusterTag)
			if err != nil {
				return nil, err
			}

			r.log.V(0).Info("Created security group", "id", *createResp.GroupId)

			// Unfortunately CreateSecurityGroup doesn't return an *ec2.SecurityGroup so just return an error to
			// put this back in the work queue after recording the security group id
			resource.Status.SecurityGroupId = *createResp.GroupId
			if err := r.Status().Update(ctx, resource); err != nil {
				return nil, fmt.Errorf("failed to update status")
			}

			return nil, fmt.Errorf("initial security group creation, reconciling again to configure")
		} else {
			sg = resp.SecurityGroups[0]
		}
	} else {
		sg = resp.SecurityGroups[0]
	}

	if sg == nil {
		return nil, fmt.Errorf("unexpectedly got a nil security group response from AWS")
	}

	r.log.V(1).Info("Found security group", "id", *resp.SecurityGroups[0].GroupId)
	resource.Status.SecurityGroupId = *sg.GroupId
	if err := r.Status().Update(ctx, resource); err != nil {
		return nil, fmt.Errorf("failed to update status")
	}

	return sg, nil
}

// createMissingSecurityGroupTags ensures the expected AWS tags exist on a VpcEndpoint CR's Security Group.
// It will not delete any extra tags and only create missing ones.
func (r *VpcEndpointReconciler) createMissingSecurityGroupTags(sg *ec2.SecurityGroup, resource *avov1alpha1.VpcEndpoint) error {
	sgName, err := util.GenerateSecurityGroupName(r.clusterInfo.infraName, resource.Name)
	if err != nil {
		return fmt.Errorf("failed to generate security group name: %v", err)
	}

	defaultTagsMap, err := util.GenerateAwsTagsAsMap(sgName, r.clusterInfo.clusterTag)
	if err != nil {
		return err
	}

	// Fix tags if any are missing
	if !tagsContains(sg.Tags, defaultTagsMap) {
		r.log.V(1).Info("Adding missing security group tags")
		defaultTags, err := util.GenerateAwsTags(sgName, r.clusterInfo.clusterTag)
		if err != nil {
			return fmt.Errorf("failed to generate expected tags: %v", err)
		}
		if _, err := r.awsClient.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{sg.GroupId},
			Tags:      defaultTags,
		}); err != nil {
			return fmt.Errorf("failed to create tags: %w", err)
		}
	}

	return nil
}

// generateMissingSecurityGroupRules ensures that the cluster's worker and master security groups are allowed ingresses
// to the VPC Endpoint security group as well as and other configured rules from the CR.
// It will not remove an extra security group rules and only create missing ones.
func (r *VpcEndpointReconciler) generateMissingSecurityGroupRules(sg *ec2.SecurityGroup, resource *avov1alpha1.VpcEndpoint) (
	*ec2.AuthorizeSecurityGroupIngressInput, *ec2.AuthorizeSecurityGroupEgressInput, error) {
	if sg == nil || resource == nil {
		return nil, nil, fmt.Errorf("security group and resource must not be nil")
	}

	rulesResp, err := r.awsClient.DescribeSecurityGroupRules(*sg.GroupId)
	if err != nil {
		return nil, nil, err
	}

	sourceSgResp, err := r.awsClient.FilterClusterNodeSecurityGroupsByDefaultTags(r.clusterInfo.infraName)
	if err != nil {
		return nil, nil, err
	}

	sourceSgIds := make([]*string, len(sourceSgResp.SecurityGroups))
	for i := range sourceSgResp.SecurityGroups {
		sourceSgIds[i] = sourceSgResp.SecurityGroups[i].GroupId
	}

	// Ensure ingress/egress rules
	var (
		ingressRules []*ec2.IpPermission
		egressRules  []*ec2.IpPermission
	)

	for i := range resource.Spec.SecurityGroup.IngressRules {
		for _, sourceSgId := range sourceSgIds {
			create := true
			for _, rule := range rulesResp.SecurityGroupRules {
				// If we find a rule with the correct protocol, fromPort, and toPort, check the source security group
				if *rule.IpProtocol == resource.Spec.SecurityGroup.IngressRules[i].Protocol &&
					*rule.FromPort == resource.Spec.SecurityGroup.IngressRules[i].FromPort &&
					*rule.ToPort == resource.Spec.SecurityGroup.IngressRules[i].ToPort &&
					*rule.ReferencedGroupInfo.GroupId == *sourceSgId {
					create = false
					break
				}
			}

			if create {
				ingressRules = append(ingressRules, &ec2.IpPermission{
					IpProtocol: aws.String(resource.Spec.SecurityGroup.IngressRules[i].Protocol),
					FromPort:   aws.Int64(resource.Spec.SecurityGroup.IngressRules[i].FromPort),
					ToPort:     aws.Int64(resource.Spec.SecurityGroup.IngressRules[i].ToPort),
					UserIdGroupPairs: []*ec2.UserIdGroupPair{
						{
							GroupId: sourceSgId,
						},
					},
				})
			}
		}
	}

	for i := range resource.Spec.SecurityGroup.EgressRules {
		for _, sourceSgId := range sourceSgIds {
			create := true
			for _, rule := range rulesResp.SecurityGroupRules {
				if *rule.IpProtocol == resource.Spec.SecurityGroup.EgressRules[i].Protocol &&
					*rule.FromPort == resource.Spec.SecurityGroup.EgressRules[i].FromPort &&
					*rule.ToPort == resource.Spec.SecurityGroup.EgressRules[i].ToPort &&
					*rule.ReferencedGroupInfo.GroupId == *sourceSgId {
					create = false
					break
				}
			}

			if create {
				egressRules = append(egressRules, &ec2.IpPermission{
					IpProtocol: aws.String(resource.Spec.SecurityGroup.EgressRules[i].Protocol),
					FromPort:   aws.Int64(resource.Spec.SecurityGroup.EgressRules[i].FromPort),
					ToPort:     aws.Int64(resource.Spec.SecurityGroup.EgressRules[i].ToPort),
					UserIdGroupPairs: []*ec2.UserIdGroupPair{
						{
							GroupId: sourceSgId,
						},
					},
				})
			}
		}
	}

	if len(ingressRules) > 0 {
		r.log.V(1).Info("Need to create ingress rules", "ingressRules", ingressRules)
	}
	if len(egressRules) > 0 {
		r.log.V(1).Info("Need to create egress rules", "egressRules", egressRules)
	}

	ingressInput := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       sg.GroupId,
		IpPermissions: ingressRules,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("security-group-rule"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(r.clusterInfo.clusterTag),
						Value: aws.String(""),
					},
					{
						Key:   aws.String(util.OperatorTagKey),
						Value: aws.String(util.OperatorTagValue),
					},
				},
			},
		},
	}

	egressInput := &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       sg.GroupId,
		IpPermissions: egressRules,
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("security-group-rule"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(r.clusterInfo.clusterTag),
						Value: aws.String(""),
					},
					{
						Key:   aws.String(util.OperatorTagKey),
						Value: aws.String(util.OperatorTagValue),
					},
				},
			},
		},
	}

	return ingressInput, egressInput, nil
}

// findOrCreateVpcEndpoint queries AWS and returns the VPC Endpoint for the provided CR and updates its status.
// It first tries to use the VPC Endpoint ID that may be in the resource's status and falls back on
// searching for the VPC Endpoint by tags in case the status is lost. If it still cannot find a VPC Endpoint,
// it gets created.
func (r *VpcEndpointReconciler) findOrCreateVpcEndpoint(ctx context.Context, resource *avov1alpha1.VpcEndpoint) (*ec2.VpcEndpoint, error) {
	var vpce *ec2.VpcEndpoint

	r.log.V(1).Info("Searching for VPC Endpoint by ID", "id", resource.Status.VPCEndpointId)
	resp, err := r.awsClient.DescribeSingleVPCEndpointById(resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
	}

	// If there's no VPC Endpoint returned by ID, look for one by tag
	// first, generate the VPC Endpoint name to search tags or use it later to create it
	if resp == nil || len(resp.VpcEndpoints) == 0 {
		vpceName, err := util.GenerateVPCEndpointName(r.clusterInfo.infraName, resource.Name)
		if err != nil {
			return nil, err
		}

		r.log.V(1).Info("Searching for VPC Endpoint by tags")
		resp, err = r.awsClient.FilterVPCEndpointByDefaultTags(r.clusterInfo.clusterTag, vpceName)
		if err != nil {
			return nil, err
		}

		// If there are still no VPC Endpoints found, it needs to be created
		if resp == nil || len(resp.VpcEndpoints) == 0 {

			creationResp, err := r.awsClient.CreateDefaultInterfaceVPCEndpoint(vpceName, r.clusterInfo.vpcId, resource.Spec.ServiceName, r.clusterInfo.clusterTag)
			if err != nil {
				return nil, fmt.Errorf("failed to create vpc endpoint: %v", err)
			}

			vpce = creationResp.VpcEndpoint
			r.log.V(0).Info("Created vpc endpoint:", "vpcEndpoint", *vpce.VpcEndpointId)
		} else {
			vpce = resp.VpcEndpoints[0]
		}
	} else {
		// There can only be one match returned by DescribeSingleVpcEndpointById
		vpce = resp.VpcEndpoints[0]
	}

	if vpce == nil {
		return nil, fmt.Errorf("unexpectedly got a nil vpce response from AWS")
	}

	resource.Status.VPCEndpointId = *vpce.VpcEndpointId
	resource.Status.Status = *vpce.State
	if err := r.Status().Update(ctx, resource); err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	return vpce, nil
}

// ensureVpcEndpointSubnets ensures that the subnets attached to the VPC Endpoint are the cluster's private subnets
func (r *VpcEndpointReconciler) ensureVpcEndpointSubnets(vpce *ec2.VpcEndpoint) error {
	subnetsToAdd, subnetsToRemove, err := r.diffVpcEndpointSubnets(vpce)
	if err != nil {
		return err
	}

	// Removing subnets first before adding to avoid
	// DuplicateSubnetsInSameZone: Found another VPC endpoint subnet in the availability zone of <existing subnet>
	if len(subnetsToRemove) > 0 {
		r.log.V(1).Info("Removing subnet(s) from VPC Endpoint", "subnetsToRemove", subnetsToRemove)
		if _, err := r.awsClient.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			RemoveSubnetIds: subnetsToRemove,
			VpcEndpointId:   vpce.VpcEndpointId,
		}); err != nil {
			return err
		}
	}

	if len(subnetsToAdd) > 0 {
		r.log.V(1).Info("Adding subnet(s) to VPC Endpoint", "subnetsToAdd", subnetsToAdd)
		if _, err := r.awsClient.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			AddSubnetIds:  subnetsToAdd,
			VpcEndpointId: vpce.VpcEndpointId,
		}); err != nil {
			return err
		}
	}

	return nil
}

// diffVpcEndpointSubnets searches for the cluster's private subnets and compares them to the subnets associated with
// the VPC Endpoint, returning subnets that need to be added to the VPC Endpoint and subnets that need to be removed
// from the VPC Endpoint.
func (r *VpcEndpointReconciler) diffVpcEndpointSubnets(vpce *ec2.VpcEndpoint) ([]*string, []*string, error) {
	if r.clusterInfo == nil || r.clusterInfo.clusterTag == "" {
		return nil, nil, fmt.Errorf("unable to parse cluster tag: %v", r.clusterInfo)
	}

	subnetsResp, err := r.awsClient.DescribePrivateSubnets(r.clusterInfo.clusterTag)
	if err != nil {
		return nil, nil, err
	}

	privateSubnetIds := make([]*string, len(subnetsResp.Subnets))
	for i := range subnetsResp.Subnets {
		privateSubnetIds[i] = subnetsResp.Subnets[i].SubnetId
	}

	subnetsToAdd, subnetsToRemove := util.StringSliceTwoWayDiff(vpce.SubnetIds, privateSubnetIds)
	return subnetsToAdd, subnetsToRemove, nil
}

// ensureVpcEndpointSecurityGroups ensures that the security group associated with the VPC Endpoint
// is only the expected one.
func (r *VpcEndpointReconciler) ensureVpcEndpointSecurityGroups(vpce *ec2.VpcEndpoint, resource *avov1alpha1.VpcEndpoint) error {
	sgToAdd, sgToRemove, err := r.diffVpcEndpointSecurityGroups(vpce, resource)
	if err != nil {
		return err
	}

	if len(sgToAdd) > 0 {
		r.log.V(1).Info("Adding security group(s) to VPC Endpoint", "sgToAdd", sgToAdd)
		if _, err := r.awsClient.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			AddSecurityGroupIds: sgToAdd,
			VpcEndpointId:       vpce.VpcEndpointId,
		}); err != nil {
			return err
		}
	}

	if len(sgToRemove) > 0 {
		r.log.V(1).Info("Removing security group(s) from VPC Endpoint", "sgToRemove", sgToRemove)
		if _, err := r.awsClient.ModifyVpcEndpoint(&ec2.ModifyVpcEndpointInput{
			RemoveSecurityGroupIds: sgToRemove,
			VpcEndpointId:          vpce.VpcEndpointId,
		}); err != nil {
			return err
		}
	}

	return nil
}

// diffVpcEndpointSecurityGroups compares the security groups associated with the VPC Endpoint with
// the security group ID recorded in the resource's status, returning security groups that need to be added
// and security groups that need to be removed from the VPC Endpoint.
func (r *VpcEndpointReconciler) diffVpcEndpointSecurityGroups(vpce *ec2.VpcEndpoint, resource *avov1alpha1.VpcEndpoint) ([]*string, []*string, error) {
	vpceSgIds := make([]*string, len(vpce.Groups))
	for i := range vpce.Groups {
		vpceSgIds[i] = vpce.Groups[i].GroupId
	}

	sgToAdd, sgToRemove := util.StringSliceTwoWayDiff(
		vpceSgIds,
		[]*string{&resource.Status.SecurityGroupId},
	)

	return sgToAdd, sgToRemove, nil
}

// generateRoute53Record generates the expected Route53 Record for a provided VpcEndpoint CR
func (r *VpcEndpointReconciler) generateRoute53Record(resource *avov1alpha1.VpcEndpoint) (*route53.ResourceRecord, error) {
	if resource.Status.VPCEndpointId == "" {
		return nil, fmt.Errorf("VPCEndpointID status is missing")
	}

	vpceResp, err := r.awsClient.DescribeSingleVPCEndpointById(resource.Status.VPCEndpointId)
	if err != nil {
		return nil, err
	}

	// VPCEndpoint doesn't exist anymore for some reason
	if vpceResp == nil || len(vpceResp.VpcEndpoints) == 0 {
		return nil, nil
	}

	// DNSEntries won't be populated until the state is available
	if *vpceResp.VpcEndpoints[0].State != "available" {
		return nil, fmt.Errorf("VPCEndpoint is not in the available state")
	}

	if len(vpceResp.VpcEndpoints[0].DnsEntries) == 0 {
		return nil, fmt.Errorf("VPCEndpoint has no DNS entries")
	}

	return &route53.ResourceRecord{
		Value: vpceResp.VpcEndpoints[0].DnsEntries[0].DnsName,
	}, nil
}

// generateExternalNameService generates the expected ExternalName service for a VpcEndpoint CustomResource
func (r *VpcEndpointReconciler) generateExternalNameService(resource *avov1alpha1.VpcEndpoint) (*corev1.Service, error) {
	if resource == nil {
		// Should never happen
		return nil, fmt.Errorf("resource must be specified")
	}

	if resource.Spec.SubdomainName == "" {
		return nil, fmt.Errorf("subdomainName is a required field")
	}

	if r.clusterInfo.domainName == "" {
		return nil, fmt.Errorf("empty domainName")
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resource.Spec.ExternalNameService.Name,
			Namespace: resource.Spec.ExternalNameService.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s", resource.Spec.SubdomainName, r.clusterInfo.domainName),
		},
	}

	if err := controllerutil.SetControllerReference(resource, svc, r.Scheme); err != nil {
		return nil, err
	}

	return svc, nil
}

// tagsContains returns true if the all the tags in tagsToCheck exist in tags
func tagsContains(tags []*ec2.Tag, tagsToCheck map[string]string) bool {
	for k, v := range tagsToCheck {
		found := false
		for _, tag := range tags {
			if *tag.Key == k && *tag.Value == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
