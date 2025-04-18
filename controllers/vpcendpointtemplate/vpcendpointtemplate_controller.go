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

package vpcendpointtemplate

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	avov1alpha2 "github.com/openshift/aws-vpce-operator/api/v1alpha2"
	hyperv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// VpcEndpointTemplateReconciler reconciles a VpcEndpointTemplate object
type VpcEndpointTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpointtemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpointtemplates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpointtemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=avo.openshift.io,resources=vpcendpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hypershift.openshift.io,resources=hostedcontrolplanes,verbs=get,list

func (r *VpcEndpointTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx).WithName("controller").WithName(ControllerName)

	vpcet := new(avov1alpha2.VpcEndpointTemplate)
	if err := r.Get(ctx, req.NamespacedName, vpcet); err != nil {
		// Ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification).
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the VpcEndpointTemplate is deleting, delete all the VpcEndpoints matching the template
	if vpcet.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(vpcet, finalizer) {
			controllerutil.AddFinalizer(vpcet, finalizer)
			if err := r.Update(ctx, vpcet); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(vpcet, finalizer) {
			vpceList := new(avov1alpha2.VpcEndpointList)
			vpceSelector, err := metav1.LabelSelectorAsSelector(&vpcet.Spec.Selector)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err := r.List(ctx, vpceList, &client.ListOptions{
				LabelSelector: vpceSelector,
			}); err != nil {
				return ctrl.Result{}, err
			}

			// Delete all VpcEndpoints matching the label selector on this VpcEndpointTemplate
			// DeleteAllOf does not act across namespaces
			// https://github.com/kubernetes-sigs/controller-runtime/issues/1842
			for i := range vpceList.Items {
				if err := r.Delete(ctx, &vpceList.Items[i]); err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		// remove our finalizer from the list and update it.
		controllerutil.RemoveFinalizer(vpcet, finalizer)
		if err := r.Update(ctx, vpcet); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Find all relevant hostedcontrolplanes
	hcpList, err := r.FilterHostedControlPlanes(ctx, vpcet)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Ensure a VpcEndpoint exists for a hostedcontrolplane
	if err := r.ValidateVpcEndpointForHostedControlPlanes(ctx, vpcet, hcpList); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VpcEndpointTemplateReconciler) ValidateVpcEndpointForHostedControlPlanes(ctx context.Context, vpcet *avov1alpha2.VpcEndpointTemplate, hcpList []hyperv1beta1.HostedControlPlane) error {
	// Any VpcEndpoint this controller creates should have this label selector
	labelSelector, err := labels.Set(vpcet.Spec.Selector.MatchLabels).AsValidatedSelector()
	if err != nil {
		return err
	}

	// We want one VpcEndpoint per namespace/HCP per vpcet
	for _, hcp := range hcpList {
		r.log.V(1).Info("Validating HostedControlPlane", "namespace", hcp.Namespace, "name", hcp.Name)

		vpceList := new(avov1alpha2.VpcEndpointList)
		if err := r.List(ctx, vpceList, &client.ListOptions{
			LabelSelector: labelSelector,
			Namespace:     hcp.Namespace,
		}); err != nil {
			return err
		}

		// If the HostedControlPlane is deleting, delete the VpcEndpoint
		if !hcp.DeletionTimestamp.IsZero() {
			for _, vpce := range vpceList.Items {
				vpce := vpce
				r.log.V(0).Info("Deleting VpcEndpoint", "namespace", vpce.Namespace, "name", vpce.Name)
				if err := r.Delete(ctx, &vpce); err != nil {
					return err
				}
			}

			continue
		}

		// Fill out a VpcEndpoint based on the HCP/namespace/awsendpointservice
		switch {
		case len(vpceList.Items) > 1:
			r.log.V(0).Info("found more than one matching VpcEndpoint, deleting extras")
			found := false
			for _, vpce := range vpceList.Items {
				vpce := vpce
				// Make sure we only keep one matching VpcEnpoint
				// If we've already found one that's matching, delete the extras
				if reflect.DeepEqual(vpce.Spec, vpcet.Spec.Template.Spec) {
					found = true
					continue
				}

				// If the VpcEndpoint doesn't match, delete it
				r.log.V(0).Info("Deleting VpcEndpoint", "namespace", vpce.Namespace, "name", vpce.Name)
				if err := r.Delete(ctx, &vpce); err != nil {
					return err
				}
			}

			if !found {
				if err := r.CreateVpcEndpoint(ctx, vpcet, hcp.Namespace); err != nil {
					return err
				}
			}

		case len(vpceList.Items) == 1:
			// Make sure the found VpcEndpoint looks good
			if err := r.ReplaceVpcEndpointSpec(ctx, &vpceList.Items[0], vpcet); err != nil {
				return err
			}
		case len(vpceList.Items) == 0:
			// Create a VpcEndpoint if none exists
			if err := r.CreateVpcEndpoint(ctx, vpcet, hcp.Namespace); err != nil {
				return err
			}
		}
	}

	return nil
}

// CreateVpcEndpoint creates a VpcEndpoint provided a vpcet and a namespace
func (r *VpcEndpointTemplateReconciler) CreateVpcEndpoint(ctx context.Context, vpcet *avov1alpha2.VpcEndpointTemplate, namespace string) error {
	vpce := new(avov1alpha2.VpcEndpoint)
	vpce.Name = vpcet.Name
	vpce.Namespace = namespace
	vpce.Labels = vpcet.Spec.Template.Labels
	vpce.Spec = *vpcet.Spec.Template.Spec.DeepCopy()

	r.log.V(0).Info("Creating VpcEndpoint", "namespace", vpce.Namespace, "name", vpce.Name)
	if err := r.Create(ctx, vpce); err != nil {
		return err
	}

	return nil
}

// ReplaceVpcEndpointSpec effectively does a "kubectl replace" if the provided actual VpcEndpoint doesn't match the vpcet
func (r *VpcEndpointTemplateReconciler) ReplaceVpcEndpointSpec(ctx context.Context, actual *avov1alpha2.VpcEndpoint, vpcet *avov1alpha2.VpcEndpointTemplate) error {
	if !reflect.DeepEqual(actual.Spec, vpcet.Spec.Template.Spec) {
		actual.Spec = *vpcet.Spec.Template.Spec.DeepCopy()
		r.log.V(0).Info("Replacing VpcEndpoint", "namespace", actual.Namespace, "name", actual.Name)
		if err := r.Update(ctx, actual); err != nil {
			return err
		}
	}

	return nil
}

// FilterHostedControlPlanes returns a list of all hostedcontrolplane resources in all namespaces.
// Basically does `oc get hostedcontrolplane -A`
// TODO: Filter over private hostedcontrolplanes in the future
func (r *VpcEndpointTemplateReconciler) FilterHostedControlPlanes(ctx context.Context, vpcet *avov1alpha2.VpcEndpointTemplate) ([]hyperv1beta1.HostedControlPlane, error) {
	if vpcet.Spec.Type != avov1alpha2.HCPVpcEndpointTemplateType {
		return nil, nil
	}

	hcpList := new(hyperv1beta1.HostedControlPlaneList)
	// Namespace represents the namespace to list for, or empty for non-namespaced objects, or to list across all namespaces.
	// i.e. Namespace: "" means list over all namespaces
	if err := r.List(ctx, hcpList, &client.ListOptions{Namespace: ""}); err != nil {
		return nil, err
	}

	return hcpList.Items, nil
}

type enquerequestForControlplane struct {
	Client client.Client
}

func (e *enquerequestForControlplane) mapAndEnqueue(ctx context.Context, q workqueue.RateLimitingInterface, obj client.Object, reqs map[reconcile.Request]struct{}) {
	// TODO: We're currently not filtering this by only Private HostedControlPlanes, so it costs us approximately $87/year/VpcEndpoint
	// Ref: https://aws.amazon.com/privatelink/pricing/
	matches := []reconcile.Request{}
	vpceTemplateList := &avov1alpha2.VpcEndpointTemplateList{}
	if err := e.Client.List(ctx, vpceTemplateList, &client.ListOptions{}); err != nil {
		return
	}

	for _, vpceTemplate := range vpceTemplateList.Items {
		if vpceTemplate.Spec.Type == avov1alpha2.HCPVpcEndpointTemplateType {
			matches = append(matches, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      vpceTemplate.Name,
					Namespace: vpceTemplate.Namespace,
				},
			})
		}
	}

	for _, req := range matches {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			// Used for de-duping requests
			reqs[req] = struct{}{}
		}
	}
}

func (e *enquerequestForControlplane) Create(ctx context.Context, evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(ctx, q, evt.Object, reqs)
}

func (e *enquerequestForControlplane) Update(ctx context.Context, evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(ctx, q, evt.ObjectOld, reqs)
	e.mapAndEnqueue(ctx, q, evt.ObjectNew, reqs)
}

func (e *enquerequestForControlplane) Delete(ctx context.Context, evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(ctx, q, evt.Object, reqs)
}

func (e *enquerequestForControlplane) Generic(ctx context.Context, evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(ctx, q, evt.Object, reqs)
}

// SetupWithManager sets up the controller with the Manager.
func (r *VpcEndpointTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&avov1alpha2.VpcEndpointTemplate{}).
		Watches(&hyperv1beta1.HostedControlPlane{}, &enquerequestForControlplane{Client: mgr.GetClient()}).
		Complete(r)
}
