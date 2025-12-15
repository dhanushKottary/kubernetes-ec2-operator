/*
Copyright 2025.

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

package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	computev1 "cloud.com/guestbook/api/v1"
)

// EC2instanceReconciler reconciles a EC2instance object
type EC2instanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compute.cloud.com,resources=ec2instances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EC2instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *EC2instanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := logf.FromContext(ctx)

	// TODO(user): your logic here
    l.Info("=== RECONCILE LOOP STARTED ===", "namespace",req.Namespace, "name", req.Name)

	ec2Instance := &computev1.EC2instance{}
	

	if err := r.Get(ctx, req.NamespacedName, ec2Instance); err != nil {
		if errors.IsNotFound(err) {
			l.Info("Instance deleted. No need to reconcile")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !ec2Instance.DeletionTimestamp.IsZero() {
		l.Info("Has delete timestamp", "instance is being deleted")
		_, err := deleteInstance(ctx, ec2Instance)
		if err != nil {
			l.Error(err, "failed to delete EC2 instances")
			return ctrl.Result{Requeue: true}, err
		}

		controllerutil.RemoveFinalizer(ec2Instance, "ec2instance.compute.cloud.com")
		if err := r.Update(ctx, ec2Instance); err != nil {
			l.Error(err, "failed to remove finalizer")
			return  ctrl.Result{Requeue: true}, err
		}

		return ctrl.Result{}, nil
	}

	if ec2Instance.Status.InstanceID != "" {
		l.Info("Requested object already exists in kubernetes. Not creating a new instance", "instanceID", ec2Instance.Status.InstanceID)
		return ctrl.Result{}, nil
	}


	l.Info("Creating a new instance")
	l.Info("=== ABOUT TO ADD FINALIZER ===")
	ec2Instance.Finalizers = append(ec2Instance.Finalizers, "ec2instance.compute.cloud.com")
	if err := r.Update(ctx, ec2Instance); err != nil {
		l.Error(err,"Failed to add Finalizer")
		return ctrl.Result{
			Requeue: true,
		}, err	
	}

	l.Info("=== FINALIZER ADDED - This update will trigger a NEW reconcile loop, but current reconcile continues ===")

	l.Info("=== CONTINUING WITH EC2 INSTANCE CREATION IN CURRENT RECONCILE ===")
	createdInstanceInfo, err := createEc2Instance(ec2Instance)
	if err != nil {
		l.Error(err, "Failed to create EC2 instance")
		// Kubernetes will retry with backoff
		return ctrl.Result{}, err
	}
    
	l.Info("=== ABOUT TO UPDATE STATUS - This will trigger reconcile loop again ===",
		"instanceID", createdInstanceInfo.InstanceID,
		"state", createdInstanceInfo.State)
	
	ec2Instance.Status.InstanceID = createdInstanceInfo.InstanceID
	ec2Instance.Status.State = createdInstanceInfo.State
	ec2Instance.Status.PublicIP = createdInstanceInfo.PublicIP
	ec2Instance.Status.PrivateIP = createdInstanceInfo.PrivateIP
	ec2Instance.Status.PublicDNS = createdInstanceInfo.PublicDNS
	ec2Instance.Status.PrivateDNS = createdInstanceInfo.PrivateDNS

	// The Reconcile function must return a ctrl.Result and an error.
	// Returning ctrl.Result{} with nil error means the reconciliation was successful
	// and no requeue is requested. If an error is returned, the controller will
	// automatically requeue the request for another attempt.
	// Sends Requeue ( bool ) and RequeueAfter ( time.Duration ).
	//return ctrl.Result{}, nil
	err = r.Status().Update(ctx, ec2Instance)
	if err != nil {
		l.Error(err, "Failed to update status")
		// Kubernetes will retry with backoff
		return ctrl.Result{}, err
	}
	l.Info("=== STATUS UPDATED - Reconcile loop will be triggered again ===")

	// Kubernetes will not retry - done, wait for next event
	return ctrl.Result{RequeueAfter: 1 * time.Second}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *EC2instanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.EC2instance{}).
		Named("ec2instance").
		Complete(r)
}
