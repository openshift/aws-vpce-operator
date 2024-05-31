//go:build osde2e

package crds

import _ "embed"

//go:embed avo.openshift.io_vpcendpoints.yaml
var VpcEndpointCRD []byte

//go:embed avo.openshift.io_vpcendpointtemplates.yaml
var VpcEndpointTemplateCRD []byte
