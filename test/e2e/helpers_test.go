// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
	"github.com/openshift/aws-vpce-operator/pkg/infrastructures"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	vpceReadyTimeout     = 10 * time.Minute
	vpceDeletedTimeout   = 5 * time.Minute
	crdEstablishTimeout  = 60 * time.Second
	crdEstablishInterval = 500 * time.Millisecond
	negativeTestTimeout  = 30 * time.Second
	pollingInterval      = 5 * time.Second
)

// awsTestHelper provides AWS SDK access for verifying resources created by the operator.
type awsTestHelper struct {
	awsClient *aws_client.AWSClient
	// ec2Client is a raw EC2 client for operations the operator's AWSClient doesn't expose
	// (e.g. creating the throwaway VPC needed to host a decoy hosted zone).
	ec2Client *ec2.Client
	region    string
}

// newAWSTestHelper initializes the helper by reading cluster info and creating an AWS client.
func newAWSTestHelper(ctx context.Context, c client.Client) *awsTestHelper {
	region, err := infrastructures.GetAWSRegion(ctx, c)
	Expect(err).ToNot(HaveOccurred(), "failed to get AWS region from infrastructure CR")

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	Expect(err).ToNot(HaveOccurred(), "failed to load default AWS config")

	return &awsTestHelper{
		awsClient: aws_client.NewAwsClient(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		region:    region,
	}
}

// testServiceNames provides unique AWS interface endpoint service names for each test to avoid
// private DNS conflicts. Only one VPC endpoint per service can exist in a VPC when private DNS
// is enabled (the AWS default for standard services).
var testServiceNames = map[string]string{
	"lifecycle":        "ssmmessages",
	"sg-rules":         "ssm",
	"cred-override":    "sqs",
	"status-reporting": "sns",
	"sg-update":        "kms",
	"drift-vpce":       "kinesis-streams",
	"tagging":          "ecr.api",
	"cleanup-atomic":   "logs",
	"decoy-zone":       "ec2messages",
	"decoy-zone-ref":   "elasticloadbalancing",
}

// testServiceName returns an AWS interface endpoint service name for the given test key and region.
func testServiceName(region, testKey string) string {
	svc, ok := testServiceNames[testKey]
	if !ok {
		svc = "ssmmessages"
	}
	return fmt.Sprintf("com.amazonaws.%s.%s", region, svc)
}

// buildVpcEndpoint returns a minimal VpcEndpoint CR for testing.
func buildVpcEndpoint(name, namespace, serviceName string) *avov1alpha2.VpcEndpoint {
	return &avov1alpha2.VpcEndpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: avov1alpha2.VpcEndpointSpec{
			ServiceName: serviceName,
			SecurityGroup: avov1alpha2.SecurityGroup{
				IngressRules: []avov1alpha2.SecurityGroupRule{
					{
						CidrIp:   "10.0.0.0/8",
						FromPort: 443,
						ToPort:   443,
						Protocol: "tcp",
					},
				},
			},
			Vpc: avov1alpha2.Vpc{
				AutoDiscoverSubnets: true,
			},
		},
	}
}

// buildVpcEndpointWithDomain returns a VpcEndpoint CR that drives the literal
// spec.customDns.route53PrivateHostedZone.domainName path, so the operator creates (or
// resolves) a Route53 private hosted zone for domainName.
func buildVpcEndpointWithDomain(name, namespace, serviceName, domainName string) *avov1alpha2.VpcEndpoint {
	vpce := buildVpcEndpoint(name, namespace, serviceName)
	vpce.Spec.CustomDns = avov1alpha2.CustomDns{
		Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
			DomainName: domainName,
			Record: avov1alpha2.Route53HostedZoneRecord{
				Hostname: "endpoint",
				ExternalNameService: avov1alpha2.ExternalNameService{
					Name: name + "-svc",
				},
			},
		},
	}
	return vpce
}

// buildVpcEndpointWithDomainNameRef returns a VpcEndpoint CR that drives the
// spec.customDns.route53PrivateHostedZone.domainNameRef.name path. Per the API, .name is a
// literal user-supplied string (api/v1alpha2/vpcendpoint_types.go: "Name specifies the base
// domain name directly") — unlike the cluster-unique valueFrom.dnsRef /
// valueFrom.hostedControlPlaneRef sources — so it carries the same shared-domain hijack risk
// as the literal domainName field and must be guarded identically.
func buildVpcEndpointWithDomainNameRef(name, namespace, serviceName, domainName string) *avov1alpha2.VpcEndpoint {
	vpce := buildVpcEndpoint(name, namespace, serviceName)
	vpce.Spec.CustomDns = avov1alpha2.CustomDns{
		Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
			DomainNameRef: &avov1alpha2.DomainName{
				Name: domainName,
			},
			Record: avov1alpha2.Route53HostedZoneRecord{
				Hostname: "endpoint",
				ExternalNameService: avov1alpha2.ExternalNameService{
					Name: name + "-svc",
				},
			},
		},
	}
	return vpce
}

// createThrowawayVPC creates a minimal VPC to host the decoy "sibling" hosted zone. A
// Route53 private hosted zone must be associated with at least one VPC, and to represent a
// sibling cluster the decoy must be associated with a VPC *other* than the cluster's. The
// operator's AWSClient can't create VPCs, so a raw EC2 client is used. Cleanup is registered
// via DeferCleanup (runs after the decoy zone is torn down, per LIFO ordering).
func (h *awsTestHelper) createThrowawayVPC(ctx context.Context) string {
	out, err := h.ec2Client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.253.0.0/28"),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeVpc,
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("avo-e2e-decoy-sibling")},
				},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred(), "failed to create throwaway VPC for decoy zone")
	vpcId := *out.Vpc.VpcId
	DeferCleanup(func(ctx context.Context) {
		if _, err := h.ec2Client.DeleteVpc(ctx, &ec2.DeleteVpcInput{VpcId: aws.String(vpcId)}); err != nil {
			GinkgoLogr.Error(err, "failed to delete throwaway VPC", "vpcId", vpcId)
		}
	})
	return vpcId
}

// createDecoyPrivateZone creates a private hosted zone for domain associated only with the
// provided (non-cluster) VPC and carrying no cluster ownership tag — a stand-in for a
// sibling cluster's zone. It seeds a sentinel record so the test can prove the operator
// never writes into it. Returns the bare hosted zone id. Cleanup via DeferCleanup.
func (h *awsTestHelper) createDecoyPrivateZone(ctx context.Context, domain, vpcId string) string {
	out, err := h.awsClient.CreateHostedZone(ctx, domain, vpcId, h.region)
	Expect(err).ToNot(HaveOccurred(), "failed to create decoy hosted zone")
	zoneId := strings.TrimPrefix(*out.HostedZone.Id, "/hostedzone/")

	// Seed a sentinel record that only the decoy owns, so the test can assert the operator
	// neither overwrote nor removed the decoy's records.
	sentinel := &route53types.ResourceRecordSet{
		Name:            aws.String(fmt.Sprintf("sentinel.%s.", domain)),
		Type:            route53types.RRTypeCname,
		TTL:             aws.Int64(300),
		ResourceRecords: []route53types.ResourceRecord{{Value: aws.String("decoy-owner.invalid")}},
	}
	_, err = h.awsClient.UpsertResourceRecordSet(ctx, sentinel, zoneId)
	Expect(err).ToNot(HaveOccurred(), "failed to seed decoy sentinel record")

	DeferCleanup(func(ctx context.Context) {
		h.cleanupHostedZone(ctx, zoneId)
	})
	return zoneId
}

// cleanupHostedZone best-effort deletes all non-SOA/NS records in a zone, then the zone.
func (h *awsTestHelper) cleanupHostedZone(ctx context.Context, zoneId string) {
	if resp, err := h.awsClient.ListResourceRecordSets(ctx, zoneId); err == nil && resp != nil {
		for i := range resp.ResourceRecordSets {
			rr := resp.ResourceRecordSets[i]
			if rr.Type == route53types.RRTypeNs || rr.Type == route53types.RRTypeSoa {
				continue
			}
			if _, err := h.awsClient.DeleteResourceRecordSet(ctx, &rr, zoneId); err != nil {
				GinkgoLogr.Error(err, "failed to delete record during decoy cleanup", "zoneId", zoneId, "record", *rr.Name)
			}
		}
	}
	if _, err := h.awsClient.DeleteHostedZone(ctx, zoneId); err != nil {
		GinkgoLogr.Error(err, "failed to delete decoy hosted zone", "zoneId", zoneId)
	}
}

// zoneAssociatedWithVPC reports whether the hosted zone is associated with vpcId.
func (h *awsTestHelper) zoneAssociatedWithVPC(ctx context.Context, zoneId, vpcId string) bool {
	resp, err := h.awsClient.GetHostedZone(ctx, zoneId)
	Expect(err).ToNot(HaveOccurred(), "failed to GetHostedZone %s", zoneId)
	for _, v := range resp.VPCs {
		if v.VPCId != nil && *v.VPCId == vpcId {
			return true
		}
	}
	return false
}

// zoneRecordNames returns the record names present in a hosted zone.
func (h *awsTestHelper) zoneRecordNames(ctx context.Context, zoneId string) []string {
	resp, err := h.awsClient.ListResourceRecordSets(ctx, zoneId)
	Expect(err).ToNot(HaveOccurred(), "failed to list records for zone %s", zoneId)
	names := make([]string, 0, len(resp.ResourceRecordSets))
	for _, rr := range resp.ResourceRecordSets {
		names = append(names, *rr.Name)
	}
	return names
}

// waitForVpceReady polls the VpcEndpoint CR until AWSVpcEndpointCondition is True
// and the status is "available". An optional timeout can be provided; if omitted,
// vpceReadyTimeout is used.
func waitForVpceReady(ctx context.Context, c client.Client, name, namespace string, timeout ...time.Duration) *avov1alpha2.VpcEndpoint {
	t := vpceReadyTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	vpce := &avov1alpha2.VpcEndpoint{}
	Eventually(func(g Gomega) {
		g.Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, vpce)).To(Succeed())
		cond := meta.FindStatusCondition(vpce.Status.Conditions, avov1alpha2.AWSVpcEndpointCondition)
		g.Expect(cond).ToNot(BeNil(), "AWSVpcEndpointCondition not set yet")
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue), "AWSVpcEndpointCondition is not True, reason: %s", cond.Reason)
		g.Expect(vpce.Status.Status).To(Equal("available"))
	}, t, pollingInterval).Should(Succeed(), "VpcEndpoint %s/%s did not become ready", namespace, name)
	return vpce
}

// waitForConditionTrue polls until a specific condition is True on the VpcEndpoint CR.
// An optional timeout can be provided; if omitted, vpceReadyTimeout is used.
func waitForConditionTrue(ctx context.Context, c client.Client, name, namespace, conditionType string, timeout ...time.Duration) *avov1alpha2.VpcEndpoint {
	t := vpceReadyTimeout
	if len(timeout) > 0 {
		t = timeout[0]
	}
	vpce := &avov1alpha2.VpcEndpoint{}
	Eventually(func(g Gomega) {
		g.Expect(c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, vpce)).To(Succeed())
		cond := meta.FindStatusCondition(vpce.Status.Conditions, conditionType)
		g.Expect(cond).ToNot(BeNil(), "%s condition not set yet", conditionType)
		g.Expect(cond.Status).To(Equal(metav1.ConditionTrue), "%s is not True, reason: %s", conditionType, cond.Reason)
	}, t, pollingInterval).Should(Succeed(), "Condition %s on %s/%s did not become True", conditionType, namespace, name)
	return vpce
}

// waitForVpceDeleted polls until the VpcEndpoint CR no longer exists.
func waitForVpceDeleted(ctx context.Context, c client.Client, name, namespace string) {
	Eventually(func(g Gomega) {
		err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &avov1alpha2.VpcEndpoint{})
		g.Expect(kerr.IsNotFound(err)).To(BeTrue(), "VpcEndpoint %s/%s still exists", namespace, name)
	}, vpceDeletedTimeout, pollingInterval).Should(Succeed(), "VpcEndpoint %s/%s was not deleted in time", namespace, name)
}

// verifyAWSVpcEndpointExists checks that a VPC Endpoint with the given ID exists in AWS.
func (h *awsTestHelper) verifyAWSVpcEndpointExists(ctx context.Context, vpceId string) {
	resp, err := h.awsClient.DescribeSingleVPCEndpointById(ctx, vpceId)
	Expect(err).ToNot(HaveOccurred())
	Expect(resp).ToNot(BeNil())
	Expect(resp.VpcEndpoints).To(HaveLen(1))
}

// verifyAWSVpcEndpointDeleted checks that a VPC Endpoint with the given ID no longer exists in AWS.
func (h *awsTestHelper) verifyAWSVpcEndpointDeleted(ctx context.Context, vpceId string) {
	Eventually(func(g Gomega) {
		resp, err := h.awsClient.DescribeSingleVPCEndpointById(ctx, vpceId)
		g.Expect(err).ToNot(HaveOccurred())
		// DescribeSingleVPCEndpointById returns nil, nil when not found
		if resp == nil {
			return
		}
		// Also handle the case where it's returned but in "deleted" state
		if len(resp.VpcEndpoints) > 0 {
			g.Expect(string(resp.VpcEndpoints[0].State)).To(Equal("deleted"),
				"VPC Endpoint %s still exists with state: %s", vpceId, resp.VpcEndpoints[0].State)
		}
	}, vpceDeletedTimeout, pollingInterval).Should(Succeed(), "VPC Endpoint %s was not deleted from AWS", vpceId)
}

// verifyAWSSecurityGroupExists checks that a security group with the given ID exists in AWS.
func (h *awsTestHelper) verifyAWSSecurityGroupExists(ctx context.Context, sgId string) {
	resp, err := h.awsClient.FilterSecurityGroupById(ctx, sgId)
	Expect(err).ToNot(HaveOccurred())
	Expect(resp).ToNot(BeNil())
	Expect(resp.SecurityGroups).ToNot(BeEmpty())
}

// verifyAWSSecurityGroupDeleted checks that a security group no longer exists in AWS.
func (h *awsTestHelper) verifyAWSSecurityGroupDeleted(ctx context.Context, sgId string) {
	Eventually(func(g Gomega) {
		resp, err := h.awsClient.FilterSecurityGroupById(ctx, sgId)
		g.Expect(err).ToNot(HaveOccurred())
		// FilterSecurityGroupById returns nil, nil for InvalidGroup.NotFound
		if resp == nil {
			return
		}
		g.Expect(resp.SecurityGroups).To(BeEmpty(), "Security group %s still exists", sgId)
	}, vpceDeletedTimeout, pollingInterval).Should(Succeed(), "Security group %s was not deleted from AWS", sgId)
}

// createTestNamespace creates a namespace for test isolation and registers cleanup.
func createTestNamespace(ctx context.Context, c client.Client, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := c.Create(ctx, ns)
	if err != nil && !kerr.IsAlreadyExists(err) {
		Expect(err).ToNot(HaveOccurred(), "failed to create test namespace %s", name)
	}
	DeferCleanup(func(ctx context.Context) {
		_ = c.Delete(ctx, ns)
	})
}

// deleteVpceAndWait deletes a VpcEndpoint CR and waits for it to be fully removed.
// If the finalizer blocks deletion beyond the timeout, it is stripped as a fallback
// following the certman-operator cleanup pattern.
func deleteVpceAndWait(ctx context.Context, c client.Client, name, namespace string) {
	vpce := &avov1alpha2.VpcEndpoint{}
	err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, vpce)
	if kerr.IsNotFound(err) {
		return
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(c.Delete(ctx, vpce)).To(Succeed())

	// Wait for the CR to be fully removed; if it gets stuck (e.g., the operator
	// can't clean up AWS resources), strip the finalizer so the test namespace
	// doesn't leak.
	deleted := isVpceDeletedWithin(ctx, c, name, namespace, vpceDeletedTimeout)
	if !deleted {
		GinkgoLogr.Info("VpcEndpoint deletion timed out, stripping finalizers", "name", name, "namespace", namespace)
		stripFinalizers(ctx, c, name, namespace)
	}
}

// isVpceDeletedWithin returns true if the CR is deleted within the timeout.
func isVpceDeletedWithin(ctx context.Context, c client.Client, name, namespace string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			return false
		case <-ticker.C:
			err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &avov1alpha2.VpcEndpoint{})
			if kerr.IsNotFound(err) {
				return true
			}
		}
	}
}

// stripFinalizers removes all finalizers from a VpcEndpoint CR to unblock deletion.
func stripFinalizers(ctx context.Context, c client.Client, name, namespace string) {
	vpce := &avov1alpha2.VpcEndpoint{}
	if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, vpce); err != nil {
		return
	}
	if len(vpce.Finalizers) > 0 {
		vpce.Finalizers = nil
		if err := c.Update(ctx, vpce); err != nil {
			GinkgoLogr.Error(err, "failed to strip finalizers", "name", name, "namespace", namespace)
		}
	}
}

// cleanupLeftover removes a VpcEndpoint CR if it exists from a previous test run.
// This makes tests idempotent when re-run on a cluster that wasn't fully cleaned up.
func cleanupLeftover(ctx context.Context, c client.Client, name, namespace string) {
	vpce := &avov1alpha2.VpcEndpoint{}
	err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, vpce)
	if kerr.IsNotFound(err) {
		return
	}
	if err != nil {
		return
	}
	GinkgoLogr.Info("cleaning up leftover VpcEndpoint from previous run", "name", name, "namespace", namespace)
	deleteVpceAndWait(ctx, c, name, namespace)
}

// getOverrideCredentials returns AWS credential override values from environment variables.
// It first checks for dedicated override env vars (AWS_ACCESS_KEY_ID_OVERRIDE,
// AWS_SECRET_ACCESS_KEY_OVERRIDE), then falls back to the default AWS credential env vars
// (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY), and finally attempts to read from the shared
// credentials file (~/.aws/credentials) using AWS_PROFILE if set.
func getOverrideCredentials() (accessKeyId, secretAccessKey string) {
	accessKeyId = os.Getenv("AWS_ACCESS_KEY_ID_OVERRIDE")
	secretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY_OVERRIDE")
	if accessKeyId != "" && secretAccessKey != "" {
		return accessKeyId, secretAccessKey
	}

	accessKeyId = os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKeyId != "" && secretAccessKey != "" {
		return accessKeyId, secretAccessKey
	}

	// Fall back to loading from the shared credentials file using AWS_PROFILE
	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		profile = "default"
	}
	cfg, err := config.LoadSharedConfigProfile(context.Background(), profile)
	if err == nil && cfg.Credentials.AccessKeyID != "" && cfg.Credentials.SecretAccessKey != "" {
		return cfg.Credentials.AccessKeyID, cfg.Credentials.SecretAccessKey
	}

	return "", ""
}
