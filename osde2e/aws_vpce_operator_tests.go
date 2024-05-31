// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	c                        client.Client
	vpceCRD, vpceTemplateCRD *apiextensionsv1.CustomResourceDefinition
)

var _ = BeforeSuite(func(ctx context.Context) {
	log.SetLogger(GinkgoLogr)
	s := runtime.NewScheme()
	Expect(apiextensionsv1.AddToScheme(s)).Should(Succeed(), "unable to register apiextensions")
	Expect(avov1alpha2.AddToScheme(s)).Should(Succeed(), "unable to register avov1alpha2")

	var err error
	c, err = client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: s,
	})
	Expect(err).ToNot(HaveOccurred(), "unable to create controller-runtime k8s client")

	// Apply CRD YAMLs to the cluster
	vpceCRD, err = applyCRDYaml(ctx, c, crds.VpcEndpointCRD)
	Expect(err).ToNot(HaveOccurred(), "failed to apply VpcEndpoint CRD")

	vpceTemplateCRD, err = applyCRDYaml(ctx, c, crds.VpcEndpointTemplateCRD)
	Expect(err).ToNot(HaveOccurred(), "failed to apply VpcEndpointTemplate CRD")

	DeferCleanup(func(ctx context.Context) {
		if err := c.Delete(ctx, vpceCRD); !kerr.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred(), "unable to delete existing VpcEndpoint CRD")
		}

		if err := c.Delete(ctx, vpceTemplateCRD); !kerr.IsNotFound(err) {
			Expect(err).ToNot(HaveOccurred(), "unable to delete existing VpcEndpointTemplate CRD")
		}
	})
})

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
		).Error().ShouldNot(HaveOccurred(), "specifying .spec.serviceName")
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
	if err := c.Create(ctx, crd); err != nil && !kerr.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to apply CRD: %v", err)
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
