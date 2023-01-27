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
	"errors"
	"fmt"
	"time"

	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	"github.com/openshift/aws-vpce-operator/controllers/util"
	"github.com/openshift/aws-vpce-operator/pkg/aws_client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// VpcEndpointReconciler reconciles a VpcEndpoint object
type VpcEndpointReconciler struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder

	log         logr.Logger
	awsClient   *aws_client.AWSClient
	clusterInfo *clusterInfo
}

// clusterInfo contains naming and AWS information unique to the cluster
type clusterInfo struct {
	// clusterTag is the tag that uniquely identifies AWS resources for this cluster
	// e.g. "kubernetes.io/cluster/${infraName}"
	clusterTag string
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
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get,list
//+kubebuilder:rbac:groups=config.openshift.io,resources=dnses,verbs=get,list
//+kubebuilder:rbac:groups=v1,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=v1,resources=services/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *VpcEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger, err := util.DefaultAVOLogger(ControllerName)
	if err != nil {
		// Shouldn't happen, but if it does, we can't log
		return ctrl.Result{}, fmt.Errorf("unable to log: %w", err)
	}

	r.log = reqLogger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	vpce := new(avov1alpha2.VpcEndpoint)
	if err := r.Get(ctx, req.NamespacedName, vpce); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.parseClusterInfo(ctx, vpce, true); err != nil {
		awsUnauthorizedOperationMetricHandler(err)
		return ctrl.Result{}, err
	}

	if vpce.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(vpce, avoFinalizer) {
			controllerutil.AddFinalizer(vpce, avoFinalizer)
			if err := r.Update(ctx, vpce); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(vpce, avoFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.cleanupAwsResources(ctx, vpce); err != nil {
				var ae smithy.APIError
				if errors.As(err, &ae) {
					// VPC Endpoints take a bit of time to delete, so if there's a dependency error,
					// we'll requeue the item, so we can try again later.
					if ae.ErrorCode() == "DependencyViolation" {
						r.log.V(0).Info("AWS dependency violation, requeueing", "error", ae.ErrorMessage())
						return ctrl.Result{RequeueAfter: time.Second * 30}, nil
					}

					awsUnauthorizedOperationMetricHandler(err)
				}

				// Catch other errors and retry
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(vpce, avoFinalizer)
			if err := r.Update(ctx, vpce); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	if err := r.validateResources(ctx, vpce,
		[]Validation{
			r.validateSecurityGroup,
			r.validateVPCEndpoint,
			r.validateCustomDns,
		}); err != nil {
		awsUnauthorizedOperationMetricHandler(err)

		return ctrl.Result{}, err
	}

	// Check again in fifteen minutes
	return ctrl.Result{RequeueAfter: time.Minute * 15}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.APIReader = mgr.GetAPIReader()

	return ctrl.NewControllerManagedBy(mgr).
		For(&avov1alpha2.VpcEndpoint{}).
		Owns(&corev1.Service{}).
		WithOptions(controller.Options{
			RateLimiter: util.DefaultAVORateLimiter(),
		}).
		Complete(r)
}
