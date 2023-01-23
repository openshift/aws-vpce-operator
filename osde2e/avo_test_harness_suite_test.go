//go:build integration
// +build integration

package exampleaddontestharness

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "github.com/openshift/aws-vpce-operator/osde2e/tests"
)

const (
	testResultsDirectory = "/test-run-results"
	jUnitOutputFilename  = "junit-example-addon.xml"
)

func TestExampleAddonTestHarness(t *testing.T) {
	RegisterFailHandler(Fail)

	suiteConfig, reporterConfig := GinkgoConfiguration()
	reporterConfig.JUnitReport = filepath.Join(testResultsDirectory, jUnitOutputFilename)
	RunSpecs(t, "AVO Test Harness", suiteConfig, reporterConfig)

}
