// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	avoNamespace    = "openshift-aws-vpce-operator"
	templateName    = "private-hcp"
	mcProbeTimeout  = 5 * time.Minute
	mcProbeInterval = 10 * time.Second
)

var _ = Describe("VpcEndpointTemplate MC Controller", Ordered, func() {
	var (
		clusterID    string
		hcpNamespace string
	)

	BeforeAll(func(ctx context.Context) {
		sharedDir := os.Getenv("SHARED_DIR")
		if sharedDir == "" {
			Skip("SHARED_DIR not set, not running in CI")
		}

		mcKubeconfig := filepath.Join(sharedDir, "hs-mc.kubeconfig")
		if _, err := os.Stat(mcKubeconfig); err != nil {
			Skip("hs-mc.kubeconfig not found in SHARED_DIR, not an MC e2e run")
		}

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

		Expect(hyperv1beta1.AddToScheme(c.Scheme())).Should(Succeed())

		clusterIDBytes, err := os.ReadFile(filepath.Join(sharedDir, "cluster-id"))
		if err != nil {
			Skip("cluster-id not found in SHARED_DIR")
		}
		clusterID = strings.TrimSpace(string(clusterIDBytes))
		if clusterID == "" {
			Skip("cluster-id is empty")
		}
		GinkgoLogr.Info("Testing against real HCP", "clusterID", clusterID)
	})

	It("finds the real HCP on the management cluster", func(ctx context.Context) {
		hcpList := &hyperv1beta1.HostedControlPlaneList{}
		err := c.List(ctx, hcpList)
		Expect(err).ToNot(HaveOccurred(), "failed to list HCPs")

		var found bool
		for _, hcp := range hcpList.Items {
			labelID := hcp.Labels["api.openshift.com/id"]
			if labelID == clusterID || hcp.Spec.ClusterID == clusterID {
				found = true
				hcpNamespace = hcp.Namespace
				GinkgoLogr.Info("Found HCP",
					"name", hcp.Name,
					"namespace", hcp.Namespace,
					"clusterID", labelID)
				break
			}
		}
		Expect(found).To(BeTrue(), "HCP with cluster ID %s not found on MC", clusterID)
	})

	It("verifies AVO created a VpcEndpoint for the real HCP", func(ctx context.Context) {
		if hcpNamespace == "" {
			Skip("HCP not found in previous test")
		}

		By("waiting for VpcEndpoint from template in HCP namespace")
		vpce := &avov1alpha2.VpcEndpoint{}
		Eventually(func(g Gomega) {
			err := c.Get(ctx, client.ObjectKey{
				Name:      templateName,
				Namespace: hcpNamespace,
			}, vpce)
			g.Expect(err).ToNot(HaveOccurred(), "VpcEndpoint not created yet")
		}, mcProbeTimeout, mcProbeInterval).Should(Succeed(),
			"AVO did not create VpcEndpoint '%s' in namespace '%s'", templateName, hcpNamespace)

		By("verifying VpcEndpoint has template labels")
		Expect(vpce.Labels).To(HaveKeyWithValue("purpose", "backplane"),
			"VpcEndpoint missing 'purpose: backplane' label from template")

		GinkgoLogr.Info("VpcEndpoint verified for real HCP",
			"name", vpce.Name,
			"namespace", vpce.Namespace,
			"labels", vpce.Labels)
	})
})
