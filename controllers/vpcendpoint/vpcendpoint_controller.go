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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	psov1alpha1 "github.com/mjlshen/pso/api/v1alpha1"
	"github.com/mjlshen/pso/pkg/aws_client"
	"github.com/mjlshen/pso/pkg/dnses"
	"github.com/mjlshen/pso/pkg/infrastructures"
	"github.com/mjlshen/pso/pkg/util"
)

const psoFinalizer = "vpcendpoint.pso.example.com/finalizer"

// VpcEndpointReconciler reconciles a VpcEndpoint object
type VpcEndpointReconciler struct {
	client.Client

	Log       logr.Logger
	Scheme    *runtime.Scheme
	AWSClient *aws_client.AWSClient

	DomainName string
	Region     string
	InfraName  string
	ClusterTag string
	VpcId      string
}

//+kubebuilder:rbac:groups=pso.example.com,resources=vpcendpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pso.example.com,resources=vpcendpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pso.example.com,resources=vpcendpoints/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get
//+kubebuilder:rbac:groups=config.openshift.io,resources=dnses,verbs=get

func (r *VpcEndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)

	region, err := infrastructures.GetAWSRegion(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.Region = region
	r.Log.V(1).Info("Parsed region from infrastructure", "region", region)

	sess, err := session.NewSession(&aws.Config{
		Region: &region,
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	r.AWSClient = aws_client.New(sess)

	infraName, err := infrastructures.GetInfrastructureName(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.InfraName = infraName
	r.Log.V(1).Info("Found infrastructure name:", "name", infraName)

	clusterTag, err := util.GetClusterTagKey(infraName)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.ClusterTag = clusterTag
	r.Log.V(1).Info("Found cluster tag:", "clusterTag", clusterTag)

	vpcId, err := r.AWSClient.GetVPCId(clusterTag)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.VpcId = vpcId
	r.Log.V(1).Info("Found vpc id:", "vpcId", vpcId)

	domainName, err := dnses.GetPrivateHostedZoneDomainName(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.DomainName = domainName
	r.Log.V(1).Info("Found domain name:", "domainName", domainName)

	pso := new(psov1alpha1.VpcEndpoint)
	if err := r.Get(ctx, req.NamespacedName, pso); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pso.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(pso, psoFinalizer) {
			controllerutil.AddFinalizer(pso, psoFinalizer)
			if err := r.Update(ctx, pso); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(pso, psoFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			if err := r.deleteAWSResources(pso); err != nil {
				if awsErr, ok := err.(awserr.Error); ok {
					// VPC Endpoints take a bit of time to delete, so if there's a dependency error,
					// we'll requeue the item, so we can try again later.
					if awsErr.Code() == "DependencyViolation" {
						r.Log.V(0).Error(awsErr, "AWS Dependency violation - requeuing")
						return ctrl.Result{RequeueAfter: time.Second * 30}, nil
					}
				}
				// Catch other errors and retry
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(pso, psoFinalizer)
			if err := r.Update(ctx, pso); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Ensure the security group is in the right state
	if _, err := r.validateSecurityGroup(ctx, r.AWSClient, pso); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	// Ensure the VPC Endpoint is in the right state
	if _, err := r.validateVPCEndpoint(ctx, r.AWSClient, pso); err != nil {
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	// TODO: Ensure the Route53 Hosted Zone record is in the right state

	// TODO: Ensure the ExternalName service is in the right state

	// Check again in 30 sec
	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcEndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&psov1alpha1.VpcEndpoint{}).
		Complete(r)
}
