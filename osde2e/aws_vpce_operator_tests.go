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
