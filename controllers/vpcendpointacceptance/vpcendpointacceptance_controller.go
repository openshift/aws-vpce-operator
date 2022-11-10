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

package vpcendpointacceptance

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/go-logr/logr"
	aaov1alpha1 "github.com/openshift/aws-account-operator/api/v1alpha1"
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/controllers/util"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// VpcEndpointAcceptanceReconciler reconciles a VpcEndpointAcceptance object
type VpcEndpointAcceptanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	log       logr.Logger
	awsClient *aws_client.VpcEndpointAcceptanceAWSClient
}

//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpointacceptances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpointacceptances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpointacceptances/finalizers,verbs=update
//+kubebuilder:rbac:groups=aws.managed.openshift.io,resources=account,verbs=get;list

func (r *VpcEndpointAcceptanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger, err := util.DefaultAVOLogger(controllerName)
	if err != nil {
		// Shouldn't happen, but if it does, we can't log
		return ctrl.Result{}, fmt.Errorf("unable to log: %w", err)
	}

	r.log = reqLogger.WithValues("Request.Name", req.Name)

	vpceAcceptance := new(avov1alpha1.VpcEndpointAcceptance)
	if err := r.Get(ctx, req.NamespacedName, vpceAcceptance); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Poll AWS for VPCE's in pendingAcceptance based on vpceAcceptance.spec.serviceIds
	region := vpceAcceptance.Spec.Region
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return ctrl.Result{}, err
	}

	// If an AssumeRoleArn is specified, sts:AssumeRole to the specified role
	if len(vpceAcceptance.Spec.AssumeRoleArn) > 0 {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), vpceAcceptance.Spec.AssumeRoleArn))
	}

	r.awsClient = aws_client.NewVpcEndpointAcceptanceAwsClient(cfg)

	// List VPC Endpoint Connections in a pendingAcceptance state
	connections, err := r.awsClient.GetVpcEndpointConnectionsPendingAcceptance(ctx, vpceAcceptance.Spec.Id)
	if err != nil {
		return ctrl.Result{}, err
	}

	var vpceToAccept []string
	for _, connection := range connections.VpcEndpointConnections {
		if vpceAcceptance.Spec.AcceptanceCriteria.AlwaysAccept {
			// Always accept VPC Endpoint connections if this option is set
			vpceToAccept = append(vpceToAccept, *connection.VpcEndpointId)

			continue
		} else if vpceAcceptance.Spec.AcceptanceCriteria.AwsAccountOperatorAccount != nil {
			// If the AwsAccountOperatorAccount acceptance criteria is specified, then
			// start generating a set of approved AWS Account IDs via listing
			// account.aws.managed.openshift.io's .spec.awsAccountId
			accounts := &aaov1alpha1.AccountList{}
			if err := r.List(ctx, accounts, client.InNamespace(vpceAcceptance.Spec.AcceptanceCriteria.AwsAccountOperatorAccount.Namespace)); err != nil {
				return ctrl.Result{}, err
			}

			validAccounts := map[string]struct{}{}
			for _, account := range accounts.Items {
				validAccounts[account.Spec.AwsAccountID] = struct{}{}
			}

			// Only accept a VPC Endpoint Connection if it's coming from an expected AWS Account
			if _, ok := validAccounts[*connection.VpcEndpointOwner]; ok {
				vpceToAccept = append(vpceToAccept, *connection.VpcEndpointId)
			}
		}
	}

	// If valid, accept the VPCE connection
	if _, err := r.awsClient.AcceptVpcEndpointConnections(ctx, vpceAcceptance.Spec.Id, vpceToAccept...); err != nil {
		return ctrl.Result{}, err
	}

	// Check again in 1 minute
	return ctrl.Result{RequeueAfter: time.Minute * 1}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcEndpointAcceptanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&avov1alpha1.VpcEndpointAcceptance{}).
		WithOptions(controller.Options{
			RateLimiter: util.DefaultAVORateLimiter(),
		}).
		Complete(r)
}
