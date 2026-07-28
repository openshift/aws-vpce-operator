// DO NOT REMOVE TAGS BELOW. IF ANY NEW TEST FILES ARE CREATED UNDER /osde2e, PLEASE ADD THESE TAGS TO THEM IN ORDER TO BE EXCLUDED FROM UNIT TESTS.
//go:build osde2e
// +build osde2e

package osde2etests

import (
	"os"
	"strings"
)

func init() {
	for _, arg := range os.Args[1:] {
		if strings.Contains(arg, "flake-attempts") {
			return
		}
	}
	v := os.Getenv("E2E_FLAKE_ATTEMPTS")
	if v == "" {
		v = "2"
	}
	os.Args = append(os.Args, "-ginkgo.flake-attempts="+v)
}
