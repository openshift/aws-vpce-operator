// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/deploy/crds"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	c           client.Client
	operatorCmd *exec.Cmd
)

var _ = BeforeSuite(func(ctx context.Context) {
	log.SetLogger(GinkgoLogr)
	s := runtime.NewScheme()
	Expect(corev1.AddToScheme(s)).Should(Succeed(), "unable to register corev1")
	Expect(apiextensionsv1.AddToScheme(s)).Should(Succeed(), "unable to register apiextensions")
	Expect(avov1alpha2.AddToScheme(s)).Should(Succeed(), "unable to register avov1alpha2")
	Expect(configv1.Install(s)).Should(Succeed(), "unable to register configv1")

	var err error
	c, err = client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: s,
	})
	Expect(err).ToNot(HaveOccurred(), "unable to create controller-runtime k8s client")

	// Apply CRD YAMLs to the cluster
	_, err = applyCRDYaml(ctx, c, crds.VpcEndpointCRD)
	Expect(err).ToNot(HaveOccurred(), "failed to apply VpcEndpoint CRD")

	_, err = applyCRDYaml(ctx, c, crds.VpcEndpointTemplateCRD)
	Expect(err).ToNot(HaveOccurred(), "failed to apply VpcEndpointTemplate CRD")

	// Build and start the operator if not already running
	if !isOperatorRunning() {
		startOperator()
	}

	DeferCleanup(func(ctx context.Context) {
		// Stop the operator subprocess. CRDs are left in place intentionally —
		// they are idempotent and removing them causes cascading failures if
		// any VpcEndpoint resources still exist or if the operator is mid-reconcile.
		stopOperator()
	})
})

// isOperatorRunning checks if the operator health endpoint is responding.
func isOperatorRunning() bool {
	resp, err := http.Get("http://localhost:8081/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// startOperator builds and starts the operator as a subprocess.
func startOperator() {
	By("building the operator binary")
	repoRoot := findRepoRoot()
	binaryPath := filepath.Join(repoRoot, "build", "aws-vpce-operator-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./main.go")
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = GinkgoWriter
	buildCmd.Stderr = GinkgoWriter
	Expect(buildCmd.Run()).To(Succeed(), "failed to build operator binary")

	By("starting the operator")
	operatorCmd = exec.Command(binaryPath)
	operatorCmd.Dir = repoRoot
	operatorCmd.Stdout = GinkgoWriter
	operatorCmd.Stderr = GinkgoWriter
	// Inherit the current environment (KUBECONFIG, AWS_PROFILE, etc.)
	operatorCmd.Env = os.Environ()

	// The operator's healthz check requires AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	// as env vars. If using AWS_PROFILE, resolve the credentials and inject them.
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		profile := os.Getenv("AWS_PROFILE")
		if profile == "" {
			profile = "default"
		}
		cfg, err := awsconfig.LoadSharedConfigProfile(context.Background(), profile)
		if err == nil && cfg.Credentials.AccessKeyID != "" && cfg.Credentials.SecretAccessKey != "" {
			operatorCmd.Env = append(operatorCmd.Env,
				"AWS_ACCESS_KEY_ID="+cfg.Credentials.AccessKeyID,
				"AWS_SECRET_ACCESS_KEY="+cfg.Credentials.SecretAccessKey,
			)
		}
	}

	Expect(operatorCmd.Start()).To(Succeed(), "failed to start operator")

	By("waiting for operator health endpoint")
	Eventually(func() error {
		resp, err := http.Get("http://localhost:8081/healthz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health endpoint returned %d", resp.StatusCode)
		}
		return nil
	}, 60*time.Second, 1*time.Second).Should(Succeed(), "operator did not become healthy")
}

// stopOperator stops the operator subprocess if it was started by the test suite.
func stopOperator() {
	if operatorCmd == nil || operatorCmd.Process == nil {
		return
	}
	By("stopping the operator")
	if err := operatorCmd.Process.Kill(); err != nil {
		GinkgoLogr.Error(err, "failed to kill operator process")
	}
	// Wait to avoid zombie processes
	_ = operatorCmd.Wait()
	operatorCmd = nil

	// Clean up the test binary
	repoRoot := findRepoRoot()
	_ = os.Remove(filepath.Join(repoRoot, "build", "aws-vpce-operator-test"))
}

// findRepoRoot walks up from the current working directory to find the repo root
// by looking for go.mod.
func findRepoRoot() string {
	dir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			Fail("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

var _ = Describe("aws-vpce-operator CEL validation", func() {
	It("accepts a valid VpcEndpoint", func(ctx context.Context) {
		Expect(c.Create(ctx, &avov1alpha2.VpcEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-vpce",
				Namespace: "default",
			},
			Spec: avov1alpha2.VpcEndpointSpec{
				ServiceName: "test",
				Vpc: avov1alpha2.Vpc{
					AutoDiscoverSubnets: true,
					Ids:                 []string{"vpc-a"},
				},
				CustomDns: avov1alpha2.CustomDns{
					Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
						Id: "ABCDEFG",
					},
				},
			},
		}, &client.CreateOptions{DryRun: []string{metav1.DryRunAll}}),
		).ShouldNot(HaveOccurred(), "specifying .spec.serviceName")

		Expect(c.Create(ctx, &avov1alpha2.VpcEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-vpce",
				Namespace: "default",
			},
			Spec: avov1alpha2.VpcEndpointSpec{
				ServiceNameRef: &avov1alpha2.ServiceName{
					Name: "test",
				},
				Vpc: avov1alpha2.Vpc{
					AutoDiscoverSubnets: true,
					Ids:                 []string{"vpc-a"},
				},
				CustomDns: avov1alpha2.CustomDns{
					Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
						Id: "ABCDEFG",
					},
				},
			},
		}, &client.CreateOptions{DryRun: []string{metav1.DryRunAll}}),
		).ShouldNot(HaveOccurred(), "specifying .spec.serviceNameRef")

		Expect(c.Create(ctx, &avov1alpha2.VpcEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-vpce",
				Namespace: "default",
			},
			Spec: avov1alpha2.VpcEndpointSpec{
				ServiceNameRef: &avov1alpha2.ServiceName{
					ValueFrom: &avov1alpha2.ServiceNameSource{
						AwsEndpointServiceRef: &avov1alpha2.AwsEndpointSelector{
							Name: "test",
						},
					},
				},
				Vpc: avov1alpha2.Vpc{
					AutoDiscoverSubnets: true,
					Ids:                 []string{"vpc-a"},
				},
				CustomDns: avov1alpha2.CustomDns{
					Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
						Id: "ABCDEFG",
					},
				},
			},
		}, &client.CreateOptions{DryRun: []string{metav1.DryRunAll}}),
		).ShouldNot(HaveOccurred(), "specifying .spec.serviceNameRef.valueFrom.awsEndpointServiceRef.name")

	})

	It("rejects VpcEndpoints that do not reference a VPC Endpoint Service", func(ctx context.Context) {
		vpce := &avov1alpha2.VpcEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-vpce-service",
				Namespace: "default",
			},
			Spec: avov1alpha2.VpcEndpointSpec{},
		}

		Expect(c.Create(ctx, vpce, &client.CreateOptions{
			DryRun: []string{metav1.DryRunAll},
		})).Should(HaveOccurred(), "able to apply VpcEndpoint without referencing a VPC Endpoint Service")
	})

	It("accepts a valid VpcEndpointTemplate", func(ctx context.Context) {
		vpceTemplate := &avov1alpha2.VpcEndpointTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "private-hcp",
				Namespace: "default",
			},
			Spec: avov1alpha2.VpcEndpointTemplateSpec{
				Type: avov1alpha2.HCPVpcEndpointTemplateType,
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{"foo": "bar"},
				},
				Template: avov1alpha2.VpceTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"foo": "bar"},
					},
					Spec: avov1alpha2.VpcEndpointSpec{
						ServiceNameRef: &avov1alpha2.ServiceName{
							ValueFrom: &avov1alpha2.ServiceNameSource{
								AwsEndpointServiceRef: &avov1alpha2.AwsEndpointSelector{
									Name: "awsendpointservice",
								},
							},
						},
						SecurityGroup: avov1alpha2.SecurityGroup{
							IngressRules: []avov1alpha2.SecurityGroupRule{
								{
									CidrIp:   "0.0.0.0/0",
									FromPort: 443,
									ToPort:   443,
									Protocol: "tcp",
								},
							},
						},
						AWSCredentialOverrideRef: &corev1.SecretReference{
							Name:      "aws-creds",
							Namespace: "default",
						},
						EnablePrivateDns: false,
						Vpc: avov1alpha2.Vpc{
							AutoDiscoverSubnets: true,
							Tags: []avov1alpha2.Tag{
								{
									Key:   "key",
									Value: "value",
								},
							},
						},
						CustomDns: avov1alpha2.CustomDns{
							Route53PrivateHostedZone: avov1alpha2.Route53PrivateHostedZone{
								AutoDiscover: false,
								AssociatedVpcs: []avov1alpha2.AssociatedVpc{
									{
										CredentialsSecretRef: &corev1.SecretReference{
											Name:      "vpc-aws-creds",
											Namespace: "default",
										},
										VpcId:  "vpc-id",
										Region: "vpc-region",
									},
								},
								DomainNameRef: &avov1alpha2.DomainName{
									ValueFrom: &avov1alpha2.DomainNameSource{
										HostedControlPlaneRef: &avov1alpha2.HostedControlPlaneSelector{
											NamespaceFieldRef: &avov1alpha2.ObjectFieldSelector{
												FieldPath: ".metadata.namespace",
											},
										},
									},
								},
								Record: avov1alpha2.Route53HostedZoneRecord{
									Hostname: "api",
								},
							},
						},
					},
				},
			},
		}

		Expect(c.Create(ctx, vpceTemplate, &client.CreateOptions{
			DryRun: []string{metav1.DryRunAll},
		})).ShouldNot(HaveOccurred(), "that is used in ROSA HCP")
	})
})

func applyCRDYaml(ctx context.Context, c client.Client, yaml []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
	obj, gvk, err := serializer.NewCodecFactory(c.Scheme()).UniversalDeserializer().Decode(yaml, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to decode VpcEndpoint CRD YAML into an object: %v", err)
	}

	if gvk.Kind != "CustomResourceDefinition" {
		return nil, fmt.Errorf("unsupported object, expected kind: CustomResourceDefinition, got: %s", gvk.Kind)
	}

	crd := obj.(*apiextensionsv1.CustomResourceDefinition)
	if err := c.Create(ctx, crd); err != nil {
		if !kerr.IsAlreadyExists(err) {
			return nil, fmt.Errorf("failed to create CRD: %v", err)
		}
		existing := &apiextensionsv1.CustomResourceDefinition{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(crd), existing); err != nil {
			return nil, fmt.Errorf("failed to get existing CRD: %v", err)
		}
		crd.ResourceVersion = existing.ResourceVersion
		if err := c.Update(ctx, crd); err != nil {
			return nil, fmt.Errorf("failed to update CRD: %v", err)
		}
	}

	// Wait for CRDs to appear in discovery, otherwise you can get responses from the API server like:
	// no matches for kind "VpcEndpoint" in version "avo.openshift.io/v1alpha2"
	envtest.WaitForCRDs(
		ctrl.GetConfigOrDie(),
		[]*apiextensionsv1.CustomResourceDefinition{crd},
		envtest.CRDInstallOptions{
			MaxTime:      60 * time.Second,
			PollInterval: 500 * time.Millisecond,
		})

	return crd, nil
}
