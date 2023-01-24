/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vpcendpoint

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	vpcePendingAcceptance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "aws_vpce_operator",
			Name:      "vpce_pendingAcceptance_total",
			Help:      "Count of VPC Endpoints in a pendingAcceptance state, labeled by name, namespace, and AWS ID",
		},
		[]string{
			"name",
			"namespace",
			"vpce_id",
		},
	)

	awsUnauthorizedOperation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "aws_vpce_operator",
			Name:      "unauthorized_operation_total",
			Help:      "Count of UnauthorizedOperation errors when making AWS API calls",
		},
		[]string{
			"action",
		},
	)
)

func init() {
	metrics.Registry.MustRegister(vpcePendingAcceptance, awsUnauthorizedOperation)
}
