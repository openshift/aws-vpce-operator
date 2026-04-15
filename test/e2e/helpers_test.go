// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
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
	vpceReadyTimeout   = 10 * time.Minute
	vpceDeletedTimeout = 5 * time.Minute
	pollingInterval    = 5 * time.Second
)

// awsTestHelper provides AWS SDK access for verifying resources created by the operator.
type awsTestHelper struct {
	awsClient *aws_client.AWSClient
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
