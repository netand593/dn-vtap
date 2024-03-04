/*
Copyright 2024.

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

	"github.com/go-logr/logr"
	networkingv1alpha1 "github.com/netand593/dn-vtap/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KokotapFinalizer = "kokotap.networking.dn-lab.io"
)

// KokotapReconciler reconciles a Kokotap object
type KokotapReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=networking.dn-lab.io,resources=kokotaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.dn-lab.io,resources=kokotaps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.dn-lab.io,resources=kokotaps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kokotap object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *KokotapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("kokotap", req.NamespacedName)

	// TODO(user): your logic here
	kokotap := &networkingv1alpha1.Kokotap{}
	err := r.Get(ctx, req.NamespacedName, kokotap)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("KokotapPod resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get KokotapPod")
		return ctrl.Result{}, err
	}
	/*
		Define the logic for the control loop, what do we want to check for the status of KokotapPod
		and hoiw will we handle that. Only deploying it, updating it and removing it?
		Are there other actions to perform???????
	*/
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KokotapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Kokotap{}).
		Complete(r)
}
