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

	vpceCleanupFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aws_vpce_operator",
			Name:      "vpce_cleanup_failure_total",
			Help:      "Count of VPC Endpoint cleanup failures during deletion, labeled by error type",
		},
		[]string{
			"error_type",
		},
	)

	vpceRoute53ReadyDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "aws_vpce_operator",
			Name:      "vpce_route53_ready_duration_seconds",
			Help:      "Time in seconds from VpcEndpoint creation to Route53 record readiness",
			Buckets:   []float64{30, 60, 120, 300, 600, 900, 1800, 3600, 5400, 7200},
		},
	)
)

func init() {
	metrics.Registry.MustRegister(vpcePendingAcceptance, awsUnauthorizedOperation, vpceCleanupFailure, vpceRoute53ReadyDuration)
}
