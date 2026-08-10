// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	avoNamespace     = "openshift-aws-vpce-operator"
	templateName     = "private-hcp"
	mcProbeTimeout   = 5 * time.Minute
	mcProbeInterval  = 10 * time.Second
	mcCleanupTimeout = 2 * time.Minute
)

var _ = Describe("VpcEndpointTemplate MC Controller", Ordered, func() {
	var (
		testNamespace string
		hcpName       string
		clusterID     string
	)

	BeforeAll(func(ctx context.Context) {
		// Verify we're on an MC by checking for the VpcEndpointTemplate
		vpcet := &avov1alpha2.VpcEndpointTemplate{}
		err := c.Get(ctx, client.ObjectKey{
			Name:      templateName,
			Namespace: avoNamespace,
		}, vpcet)
		if kerr.IsNotFound(err) {
			Skip("VpcEndpointTemplate 'private-hcp' not found, not running on an MC")
		}
		Expect(err).ToNot(HaveOccurred(), "failed to get VpcEndpointTemplate")
		GinkgoLogr.Info("Found VpcEndpointTemplate", "name", templateName, "type", vpcet.Spec.Type)

		// Verify HyperShift scheme is registered
		Expect(hyperv1beta1.AddToScheme(c.Scheme())).Should(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		if testNamespace == "" {
			return
		}
		hcp := &hyperv1beta1.HostedControlPlane{}
		if err := c.Get(ctx, client.ObjectKey{Name: hcpName, Namespace: testNamespace}, hcp); err == nil {
			Expect(c.Delete(ctx, hcp)).To(Succeed())
		}
		ns := &corev1.Namespace{}
		if err := c.Get(ctx, client.ObjectKey{Name: testNamespace}, ns); err == nil {
			Expect(c.Delete(ctx, ns)).To(Succeed())
		}
	})

	It("creates VpcEndpoint for a new HostedControlPlane", func(ctx context.Context) {
		clusterID = fmt.Sprintf("test-avo-mc-%d", time.Now().Unix())
		hcpName = fmt.Sprintf("avo-hcp-%d", time.Now().UnixNano()%100000)
		testNamespace = fmt.Sprintf("clusters-%s-%s", clusterID, hcpName)

		By(fmt.Sprintf("creating test namespace: %s", testNamespace))
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
		Expect(c.Create(ctx, ns)).To(Succeed())

		By("creating HostedControlPlane")
		hcp := buildMCHCP(clusterID, hcpName, testNamespace, hyperv1beta1.Public)
		Expect(c.Create(ctx, hcp)).To(Succeed(), "failed to create HCP")

		By("waiting for AVO to create VpcEndpoint from template")
		vpce := &avov1alpha2.VpcEndpoint{}
		Eventually(func(g Gomega) {
			err := c.Get(ctx, client.ObjectKey{
				Name:      templateName,
				Namespace: testNamespace,
			}, vpce)
			g.Expect(err).ToNot(HaveOccurred(), "VpcEndpoint not created yet")
		}, mcProbeTimeout, mcProbeInterval).Should(Succeed(),
			"AVO did not create VpcEndpoint '%s' in namespace '%s'", templateName, testNamespace)

		By("verifying VpcEndpoint has template labels")
		Expect(vpce.Labels).To(HaveKeyWithValue("purpose", "backplane"),
			"VpcEndpoint missing 'purpose: backplane' label from template")

		GinkgoLogr.Info("VpcEndpoint created by template controller",
			"name", vpce.Name,
			"namespace", vpce.Namespace,
			"labels", vpce.Labels)
	})

	It("cleans up VpcEndpoint when HostedControlPlane is deleted", func(ctx context.Context) {
		if testNamespace == "" {
			Skip("previous test did not run")
		}

		By("deleting the HostedControlPlane")
		hcp := &hyperv1beta1.HostedControlPlane{}
		err := c.Get(ctx, client.ObjectKey{Name: hcpName, Namespace: testNamespace}, hcp)
		Expect(err).ToNot(HaveOccurred())
		Expect(c.Delete(ctx, hcp)).To(Succeed())

		By("waiting for VpcEndpoint to be cleaned up")
		Eventually(func(g Gomega) {
			err := c.Get(ctx, client.ObjectKey{
				Name:      templateName,
				Namespace: testNamespace,
			}, &avov1alpha2.VpcEndpoint{})
			g.Expect(kerr.IsNotFound(err)).To(BeTrue(),
				"VpcEndpoint still exists after HCP deletion")
		}, mcCleanupTimeout, mcProbeInterval).Should(Succeed(),
			"VpcEndpoint was not cleaned up after HCP deletion")

		GinkgoLogr.Info("VpcEndpoint cleaned up after HCP deletion")
	})
})

func buildMCHCP(clusterID, name, namespace string, endpointAccess hyperv1beta1.AWSEndpointAccessType) *hyperv1beta1.HostedControlPlane {
	hostname := fmt.Sprintf("api.%s.test.devshift.org", name)
	return &hyperv1beta1.HostedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"api.openshift.com/id":   clusterID,
				"api.openshift.com/name": name,
				"test-type":              "osde2e-avo-mc",
			},
		},
		Spec: hyperv1beta1.HostedControlPlaneSpec{
			ClusterID: clusterID,
			InfraID:   clusterID,
			Platform: hyperv1beta1.PlatformSpec{
				Type: hyperv1beta1.AWSPlatform,
				AWS: &hyperv1beta1.AWSPlatformSpec{
					Region:         "us-west-2",
					EndpointAccess: endpointAccess,
				},
			},
			IssuerURL:  "https://kubernetes.default.svc",
			PullSecret: corev1.LocalObjectReference{Name: "pull-secret"},
			SSHKey:     corev1.LocalObjectReference{Name: "ssh-key"},
			DNS: hyperv1beta1.DNSSpec{
				BaseDomain: fmt.Sprintf("%s.test.devshift.org", name),
			},
			Etcd: hyperv1beta1.EtcdSpec{
				ManagementType: hyperv1beta1.Managed,
				Managed: &hyperv1beta1.ManagedEtcdSpec{
					Storage: hyperv1beta1.ManagedEtcdStorageSpec{
						Type: hyperv1beta1.PersistentVolumeEtcdStorage,
					},
				},
			},
			Services: []hyperv1beta1.ServicePublishingStrategyMapping{
				{
					Service: hyperv1beta1.APIServer,
					ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
						Type: hyperv1beta1.Route,
						Route: &hyperv1beta1.RoutePublishingStrategy{
							Hostname: hostname,
						},
					},
				},
				{
					Service: hyperv1beta1.OAuthServer,
					ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
						Type: hyperv1beta1.Route,
					},
				},
				{
					Service: hyperv1beta1.Konnectivity,
					ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
						Type: hyperv1beta1.Route,
					},
				},
				{
					Service: hyperv1beta1.Ignition,
					ServicePublishingStrategy: hyperv1beta1.ServicePublishingStrategy{
						Type: hyperv1beta1.Route,
					},
				},
			},
		},
	}
}
