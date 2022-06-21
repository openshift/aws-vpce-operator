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
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/go-logr/logr"
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"
)

// VpcEndpointReconciler reconciles a VpcEndpoint object
type VpcEndpointReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	log         logr.Logger
	awsClient   *aws_client.AWSClient
	clusterInfo *clusterInfo
}

// clusterInfo contains naming and AWS information unique to the cluster
type clusterInfo struct {
	// clusterTag is the tag that uniquely identifies AWS resources for this cluster
	// e.g. "k8s.io/cluster/${infraName}"
	clusterTag string
	// domainName is the domain name for the cluster's private hosted zone
	// e.g. "${clusterName}.abcd.s1.devshift.org"
	domainName string
	// infraName is the name shown in the cluster's infrastructures CR
	// e.g. "${clusterName}-abcd"
	infraName string
	// region is the AWS region for the cluster
	region string
	// vpcId is the AWS VPC ID the cluster resides in
	vpcId string
}

//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpoints/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get
//+kubebuilder:rbac:groups=config.openshift.io,resources=dnses,verbs=get

func (r *VpcEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger, err := defaultAVOLogger()
	if err != nil {
		// Shouldn't happen, but if it does, we can't log
		return ctrl.Result{}, err
	}

	r.log = reqLogger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	if err := r.parseClusterInfo(ctx, true); err != nil {
		return ctrl.Result{}, err
	}

	avo := new(avov1alpha1.VpcEndpoint)
	if err := r.Get(ctx, req.NamespacedName, avo); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if avo.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(avo, avoFinalizer) {
			controllerutil.AddFinalizer(avo, avoFinalizer)
			if err := r.Update(ctx, avo); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(avo, avoFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteAWSResources(ctx, avo); err != nil {
				if awsErr, ok := err.(awserr.Error); ok {
					// VPC Endpoints take a bit of time to delete, so if there's a dependency error,
					// we'll requeue the item, so we can try again later.
					if awsErr.Code() == "DependencyViolation" {
						r.log.V(0).Info("AWS dependency violation, requeueing", "error", awsErr.Message())
						return ctrl.Result{RequeueAfter: time.Second * 30}, nil
					}
				}
				// Catch other errors and retry
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(avo, avoFinalizer)
			if err := r.Update(ctx, avo); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.validateAWSResources(ctx, avo,
		[]ValidateAWSResourceFunc{
			r.validateSecurityGroup,
			r.validateVPCEndpoint,
			r.validateR53HostedZoneRecord,
		}); err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Ensure the ExternalName service is in the right state

	// Check again in 30 sec
	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&avov1alpha1.VpcEndpoint{}).
		WithOptions(controller.Options{
			RateLimiter: defaultAVORateLimiter(),
		}).
		Complete(r)
}
