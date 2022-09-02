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

	"github.com/go-logr/logr"
	avov1alpha1 "github.com/openshift/aws-vpce-operator/api/v1alpha1"
	"github.com/openshift/aws-vpce-operator/controllers/util"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// VpcEndpointAcceptanceReconciler reconciles a VpcEndpointAcceptance object
type VpcEndpointAcceptanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	log logr.Logger
}

//+kubebuilder:rbac:groups=avo.avo.openshift.io,resources=vpcendpointacceptances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=avo.avo.openshift.io,resources=vpcendpointacceptances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=avo.avo.openshift.io,resources=vpcendpointacceptances/finalizers,verbs=update

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

	// Setup a map of AWS Account IDs:Namespaces
	// List account.aws.managed.openshift.io's .spec.awsAccountId

	// Connect to the cluster in that namespace to see if it's a valid request

	// If valid, accept the VPCE connection

	// Check again in 30 sec
	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
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
