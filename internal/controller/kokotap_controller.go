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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// +kubebuilder:scaffold:imports

	networkingv1alpha1 "github.com/netand593/dn-vtap/api/v1alpha1"
)

// KokotapReconciler reconciles a Kokotap object
type KokotapReconciler struct {
	client.Client
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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

func (r *KokotapReconciler) ReconcileNormal(ctx context.Context, req ctrl.Request, vtap *networkingv1alpha1.Kokotap) (ctrl.Result, error) {
	// Fetch the Kokotap instance
	/* instance := &networkingv1alpha1.Kokotap{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	*/

	// Logic shall be added here

	var pod corev1.Pod
	podName := vtap.Spec.PodName

	// Check if the pod exists

	err := r.Get(ctx, types.NamespacedName{Name: vtap.Spec.PodName, Namespace: vtap.Spec.Namespace}, &pod)
	logger := log.FromContext(ctx)
	if err != nil && client.IgnoreNotFound(err) != nil {
		// Pod does not exist, create it
		pod = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: vtap.Namespace,
				Labels: map[string]string{
					"dn-vtap": "tapped",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "kokotap-network-tap",
						Image: "quay.io/s1061123/kokotap:latest",

						Args: []string{
							"--procprefix=/hostproc",
							"mode",
							"sender",
							//TODO: Add a function to get containerID from podName
							"--containerid=cri-o://bba6f1800fc4a5bcd9b7f5df68f083c17076250124cbd8ebaf9a0d1868b12b8c",
							"--mirrortype=" + vtap.Spec.MirrorType,
							"--ifname= mirror",
							"--mirrorif=" + vtap.Spec.PodInterface,
							"--pod=" + vtap.Spec.PodName,
							"--namespace=" + vtap.Namespace,
							//TODO: Add a function to get node's IP given podName (see kokotap)
							"--vxlan-egressip=192.168.1.108",
							"--vxlan-ip=" + vtap.Spec.TargetIP,
							"--vxlan-id=" + vtap.Spec.VxLANID,
							"--vxlan-port=4789",
						},
					},
				},
			},
		}
	}
	if err := r.Create(ctx, &pod); err != nil {
		logger.Error(err, "Failed to create Pod", "Pod.Namespace", vtap.Namespace, "Pod.Name", podName)
		return (ctrl.Result{}), err
	}
	logger.Info("Created Pod successfully", "Pod.Namespace", vtap.Namespace, "Pod.Name", podName)
	return ctrl.Result{}, nil
	/* else if err != nil {
		logger.Error(err, "Failed to get Pod")
		return (ctrl.Result{}), err
	}
	*/
}

// SetupWithManager sets up the controller with the Manager.
func (r *KokotapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Kokotap{}).
		Complete(r)
}
