// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("aws-vpce-operator", func() {
	ginkgo.It("Placeholder avo test", func(ctx context.Context) {
		Expect(true).NotTo(Equal(false))
	})
})
