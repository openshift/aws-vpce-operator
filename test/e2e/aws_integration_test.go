// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("aws-vpce-operator AWS integration", func() {
	var (
		helper *awsTestHelper
		ns     string
	)

	BeforeEach(func(ctx context.Context) {
		if operatorCmd == nil && !isOperatorRunning() {
			dep := &unstructured.Unstructured{}
			dep.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			})
			err := c.Get(ctx, client.ObjectKey{
				Name:      "aws-vpce-operator",
				Namespace: "openshift-aws-vpce-operator",
			}, dep)
			if err != nil {
				Skip("aws-vpce-operator deployment not found in openshift-aws-vpce-operator namespace")
			}
		}

		helper = newAWSTestHelper(ctx, c)
		ns = "default"
	})

	// Test Suite 1: VPC Endpoint lifecycle (create → verify → delete → verify cleanup)
	Describe("VPC Endpoint lifecycle", Ordered, func() {
		const vpceName = "e2e-lifecycle"
		var (
			vpceId string
			sgId   string
		)

		It("should create a VPC Endpoint and reach available state", func(ctx context.Context) {
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, testServiceName(helper.region, "lifecycle"))
			Expect(c.Create(ctx, vpce)).To(Succeed(), "failed to create VpcEndpoint CR")
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			// Wait for the operator to reconcile the CR to ready state
			readyVpce := waitForVpceReady(ctx, c, vpceName, ns)

			// Capture IDs for subsequent assertions
			vpceId = readyVpce.Status.VPCEndpointId
			sgId = readyVpce.Status.SecurityGroupId

			By("verifying status fields are populated")
			Expect(vpceId).ToNot(BeEmpty(), ".status.vpcEndpointId should be set")
			Expect(sgId).ToNot(BeEmpty(), ".status.securityGroupId should be set")
			Expect(readyVpce.Status.VPCId).ToNot(BeEmpty(), ".status.vpcId should be set")
			Expect(readyVpce.Status.VPCEndpointServiceName).To(Equal(testServiceName(helper.region, "lifecycle")))
			Expect(readyVpce.Status.Status).To(Equal("available"))

			By("verifying VPC Endpoint exists in AWS")
			helper.verifyAWSVpcEndpointExists(ctx, vpceId)

			By("verifying Security Group exists in AWS")
			helper.verifyAWSSecurityGroupExists(ctx, sgId)
		})

		It("should delete AWS resources when the CR is deleted", func(ctx context.Context) {
			// vpceId and sgId are populated by the previous Ordered test
			Expect(vpceId).ToNot(BeEmpty(), "vpceId must be set from previous test")
			Expect(sgId).ToNot(BeEmpty(), "sgId must be set from previous test")

			By("deleting the VpcEndpoint CR")
			vpce := &avov1alpha2.VpcEndpoint{}
			err := c.Get(ctx, client.ObjectKey{Name: vpceName, Namespace: ns}, vpce)
			if kerr.IsNotFound(err) {
				// CR was already cleaned up (e.g., by DeferCleanup); skip to AWS verification
				GinkgoLogr.Info("VpcEndpoint CR already deleted, verifying AWS cleanup", "name", vpceName)
			} else {
				Expect(err).ToNot(HaveOccurred())
				Expect(c.Delete(ctx, vpce)).To(Succeed())
				By("waiting for the CR to be fully removed (finalizer ran)")
				waitForVpceDeleted(ctx, c, vpceName, ns)
			}

			By("verifying VPC Endpoint is deleted from AWS")
			helper.verifyAWSVpcEndpointDeleted(ctx, vpceId)

			By("verifying Security Group is deleted from AWS")
			helper.verifyAWSSecurityGroupDeleted(ctx, sgId)
		})
	})

	// Test Suite 2: Security Group reconciliation
	Describe("Security Group reconciliation", func() {
		const vpceName = "e2e-sg-rules"

		It("should create a security group with specified ingress rules", func(ctx context.Context) {
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, testServiceName(helper.region, "sg-rules"))
			vpce.Spec.SecurityGroup = avov1alpha2.SecurityGroup{
				IngressRules: []avov1alpha2.SecurityGroupRule{
					{
						CidrIp:   "10.0.0.0/8",
						FromPort: 443,
						ToPort:   443,
						Protocol: "tcp",
					},
				},
				EgressRules: []avov1alpha2.SecurityGroupRule{
					{
						CidrIp:   "0.0.0.0/0",
						FromPort: 443,
						ToPort:   443,
						Protocol: "tcp",
					},
				},
			}

			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			By("waiting for AWSSecurityGroupCondition to be True")
			readyVpce := waitForConditionTrue(ctx, c, vpceName, ns, avov1alpha2.AWSSecurityGroupCondition)
			sgId := readyVpce.Status.SecurityGroupId
			Expect(sgId).ToNot(BeEmpty())

			By("verifying security group rules exist in AWS")
			rulesResp, err := helper.awsClient.DescribeSecurityGroupRules(ctx, sgId)
			Expect(err).ToNot(HaveOccurred())
			Expect(rulesResp.SecurityGroupRules).ToNot(BeEmpty())

			// Verify we have at least one ingress rule matching our spec
			foundIngress := false
			for _, rule := range rulesResp.SecurityGroupRules {
				if rule.IsEgress != nil && !*rule.IsEgress &&
					rule.CidrIpv4 != nil && *rule.CidrIpv4 == "10.0.0.0/8" &&
					rule.FromPort != nil && *rule.FromPort == 443 &&
					rule.ToPort != nil && *rule.ToPort == 443 {
					foundIngress = true
					break
				}
			}
			Expect(foundIngress).To(BeTrue(), "expected ingress rule 10.0.0.0/8:443 not found in security group %s", sgId)

			// Verify we have at least one egress rule matching our spec
			foundEgress := false
			for _, rule := range rulesResp.SecurityGroupRules {
				if rule.IsEgress != nil && *rule.IsEgress &&
					rule.CidrIpv4 != nil && *rule.CidrIpv4 == "0.0.0.0/0" &&
					rule.FromPort != nil && *rule.FromPort == 443 &&
					rule.ToPort != nil && *rule.ToPort == 443 {
					foundEgress = true
					break
				}
			}
			Expect(foundEgress).To(BeTrue(), "expected egress rule 0.0.0.0/0:443 not found in security group %s", sgId)
		})
	})

	// Test Suite 3: AWS Credential Override
	Describe("AWS Credential Override", func() {
		const vpceName = "e2e-cred-override"

		It("should use credential override from a referenced secret", func(ctx context.Context) {
			accessKeyId, secretAccessKey := getOverrideCredentials()
			Expect(accessKeyId).ToNot(BeEmpty(), "AWS credentials not found: set AWS_ACCESS_KEY_ID_OVERRIDE, AWS_ACCESS_KEY_ID, or configure AWS_PROFILE")
			Expect(secretAccessKey).ToNot(BeEmpty(), "AWS credentials not found: set AWS_SECRET_ACCESS_KEY_OVERRIDE, AWS_SECRET_ACCESS_KEY, or configure AWS_PROFILE")

			testNs := "avo-e2e-cred-override"
			createTestNamespace(ctx, c, testNs)

			By("creating an AWS credentials secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "e2e-aws-creds",
					Namespace: testNs,
				},
				Data: map[string][]byte{
					"aws_access_key_id":     []byte(accessKeyId),
					"aws_secret_access_key": []byte(secretAccessKey),
				},
			}
			Expect(c.Create(ctx, secret)).To(Succeed())

			By("creating a VpcEndpoint CR with credential override")
			cleanupLeftover(ctx, c, vpceName, testNs)
			vpce := buildVpcEndpoint(vpceName, testNs, testServiceName(helper.region, "cred-override"))
			vpce.Spec.AWSCredentialOverrideRef = &corev1.SecretReference{
				Name:      "e2e-aws-creds",
				Namespace: testNs,
			}
			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, testNs)
			})

			By("waiting for VPC Endpoint to become ready")
			readyVpce := waitForVpceReady(ctx, c, vpceName, testNs)

			Expect(readyVpce.Status.VPCEndpointId).ToNot(BeEmpty(),
				"VPCE should be created using override credentials")

			By("verifying VPC Endpoint exists in AWS")
			helper.verifyAWSVpcEndpointExists(ctx, readyVpce.Status.VPCEndpointId)
		})
	})

	// Test Suite 4: Security Group update
	Describe("Security Group update", func() {
		const vpceName = "e2e-sg-update"

		It("should add new ingress rules when the CR spec is updated", func(ctx context.Context) {
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, testServiceName(helper.region, "sg-update"))
			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			By("waiting for initial VPCE to become ready")
			readyVpce := waitForVpceReady(ctx, c, vpceName, ns)
			sgId := readyVpce.Status.SecurityGroupId
			Expect(sgId).ToNot(BeEmpty())

			By("adding a second ingress rule to the CR spec")
			current := &avov1alpha2.VpcEndpoint{}
			Expect(c.Get(ctx, client.ObjectKey{Name: vpceName, Namespace: ns}, current)).To(Succeed())
			current.Spec.SecurityGroup.IngressRules = append(current.Spec.SecurityGroup.IngressRules,
				avov1alpha2.SecurityGroupRule{
					CidrIp:   "172.16.0.0/12",
					FromPort: 8443,
					ToPort:   8443,
					Protocol: "tcp",
				},
			)
			Expect(c.Update(ctx, current)).To(Succeed())

			By("verifying the new rule appears in AWS")
			Eventually(func(g Gomega) {
				rulesResp, err := helper.awsClient.DescribeSecurityGroupRules(ctx, sgId)
				g.Expect(err).ToNot(HaveOccurred())
				found := false
				for _, rule := range rulesResp.SecurityGroupRules {
					if rule.IsEgress != nil && !*rule.IsEgress &&
						rule.CidrIpv4 != nil && *rule.CidrIpv4 == "172.16.0.0/12" &&
						rule.FromPort != nil && *rule.FromPort == 8443 &&
						rule.ToPort != nil && *rule.ToPort == 8443 {
						found = true
						break
					}
				}
				g.Expect(found).To(BeTrue(), "new ingress rule 172.16.0.0/12:8443 not found in SG %s", sgId)
			}, vpceReadyTimeout, pollingInterval).Should(Succeed())
		})
	})

	// Test Suite 5: Drift detection
	Describe("Drift detection", func() {
		It("should recreate a manually deleted VPC endpoint", func(ctx context.Context) {
			const vpceName = "e2e-drift-vpce"
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, testServiceName(helper.region, "drift-vpce"))
			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			By("waiting for VPCE to become ready")
			readyVpce := waitForVpceReady(ctx, c, vpceName, ns)
			originalVpceId := readyVpce.Status.VPCEndpointId
			Expect(originalVpceId).ToNot(BeEmpty())

			By("manually deleting the VPC endpoint via AWS SDK")
			_, err := helper.awsClient.DeleteVPCEndpoint(ctx, originalVpceId)
			Expect(err).ToNot(HaveOccurred(), "failed to delete VPC endpoint %s", originalVpceId)

			By("waiting for the operator to detect drift and recreate the VPC endpoint")
			Eventually(func(g Gomega) {
				obj := &avov1alpha2.VpcEndpoint{}
				g.Expect(c.Get(ctx, client.ObjectKey{Name: vpceName, Namespace: ns}, obj)).To(Succeed())
				g.Expect(obj.Status.VPCEndpointId).ToNot(BeEmpty())
				g.Expect(obj.Status.VPCEndpointId).ToNot(Equal(originalVpceId),
					"operator has not replaced the deleted VPC endpoint yet")
				resp, err := helper.awsClient.DescribeSingleVPCEndpointById(ctx, obj.Status.VPCEndpointId)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(resp).ToNot(BeNil())
				g.Expect(resp.VpcEndpoints).To(HaveLen(1))
				g.Expect(string(resp.VpcEndpoints[0].State)).To(Equal("available"),
					"VPC endpoint %s is not yet available", obj.Status.VPCEndpointId)
			}, vpceReadyTimeout, pollingInterval).Should(Succeed(), "operator did not recreate the VPC endpoint")
		})
	})

	// Test Suite 6: Finalizer and tagging
	Describe("Finalizer and tagging", func() {
		const vpceName = "e2e-tagging"

		It("should set the operator finalizer and tag AWS resources", func(ctx context.Context) {
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, testServiceName(helper.region, "tagging"))
			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			By("waiting for VPCE to become ready")
			readyVpce := waitForVpceReady(ctx, c, vpceName, ns)

			By("verifying the finalizer is set on the CR")
			Expect(readyVpce.Finalizers).To(ContainElement("vpcendpoint.avo.openshift.io/finalizer"),
				"operator finalizer should be present on VpcEndpoint CR")

			By("verifying operator tag on the VPC endpoint")
			vpceResp, err := helper.awsClient.DescribeSingleVPCEndpointById(ctx, readyVpce.Status.VPCEndpointId)
			Expect(err).ToNot(HaveOccurred())
			Expect(vpceResp.VpcEndpoints).To(HaveLen(1))
			foundOperatorTag := false
			for _, tag := range vpceResp.VpcEndpoints[0].Tags {
				if tag.Key != nil && *tag.Key == "kubernetes.io/aws-vpce-operator" &&
					tag.Value != nil && *tag.Value == "managed" {
					foundOperatorTag = true
					break
				}
			}
			Expect(foundOperatorTag).To(BeTrue(),
				"VPC endpoint %s missing operator tag kubernetes.io/aws-vpce-operator=managed", readyVpce.Status.VPCEndpointId)

			By("verifying operator tag on the security group")
			sgResp, err := helper.awsClient.FilterSecurityGroupById(ctx, readyVpce.Status.SecurityGroupId)
			Expect(err).ToNot(HaveOccurred())
			Expect(sgResp.SecurityGroups).ToNot(BeEmpty())
			foundSgTag := false
			for _, tag := range sgResp.SecurityGroups[0].Tags {
				if tag.Key != nil && *tag.Key == "kubernetes.io/aws-vpce-operator" &&
					tag.Value != nil && *tag.Value == "managed" {
					foundSgTag = true
					break
				}
			}
			Expect(foundSgTag).To(BeTrue(),
				"Security group %s missing operator tag kubernetes.io/aws-vpce-operator=managed", readyVpce.Status.SecurityGroupId)
		})
	})

	// Test Suite 7: Error scenarios
	Describe("Error scenarios", func() {
		It("should handle an invalid VPC Endpoint Service name gracefully", func(ctx context.Context) {
			const vpceName = "e2e-invalid-svc"
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, "com.amazonaws.vpce-svc.doesnotexist")
			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			By("verifying the VPC Endpoint never reaches available state")
			Consistently(func(g Gomega) {
				obj := &avov1alpha2.VpcEndpoint{}
				err := c.Get(ctx, client.ObjectKey{Name: vpceName, Namespace: ns}, obj)
				if kerr.IsNotFound(err) {
					return
				}
				g.Expect(err).ToNot(HaveOccurred())

				// The VPCE should either have no condition or a False condition
				cond := meta.FindStatusCondition(obj.Status.Conditions, avov1alpha2.AWSVpcEndpointCondition)
				if cond != nil {
					g.Expect(cond.Status).ToNot(Equal(metav1.ConditionTrue),
						"VPC Endpoint should not become ready with invalid service name")
				}
			}, "30s", pollingInterval).Should(Succeed())
		})

		It("should report VPC Endpoint status transitions", func(ctx context.Context) {
			const vpceName = "e2e-status-reporting"
			cleanupLeftover(ctx, c, vpceName, ns)
			vpce := buildVpcEndpoint(vpceName, ns, testServiceName(helper.region, "status-reporting"))
			Expect(c.Create(ctx, vpce)).To(Succeed())
			DeferCleanup(func(ctx context.Context) {
				deleteVpceAndWait(ctx, c, vpceName, ns)
			})

			By("verifying status is set during reconciliation")
			Eventually(func(g Gomega) {
				obj := &avov1alpha2.VpcEndpoint{}
				g.Expect(c.Get(ctx, client.ObjectKey{Name: vpceName, Namespace: ns}, obj)).To(Succeed())
				// The status should be set to something (pending, available, etc.)
				g.Expect(obj.Status.Status).ToNot(BeEmpty(), ".status.status should be reported")
			}, vpceReadyTimeout, pollingInterval).Should(Succeed())

			By("verifying the endpoint eventually becomes available")
			readyVpce := waitForVpceReady(ctx, c, vpceName, ns)
			Expect(readyVpce.Status.Status).To(Equal("available"))

			By("verifying all expected conditions are set")
			Expect(meta.FindStatusCondition(readyVpce.Status.Conditions, avov1alpha2.AWSVpcEndpointCondition)).ToNot(BeNil())
			Expect(meta.FindStatusCondition(readyVpce.Status.Conditions, avov1alpha2.AWSSecurityGroupCondition)).ToNot(BeNil())
		})
	})
})
