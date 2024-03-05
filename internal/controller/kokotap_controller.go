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
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	networkingv1alpha1 "github.com/netand593/dn-vtap/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	// Create a new variable of type Kokotap
	kokotap := &networkingv1alpha1.Kokotap{}
	err := r.Get(ctx, req.NamespacedName, kokotap)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Kokotap resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Kokotap")
		return ctrl.Result{}, err
	}
	/*
		Define the logic for the control loop, what do we want to check for the status of KokotapPod
		and hoiw will we handle that. Only deploying it, updating it and removing it?
		Are there other actions to perform???????
	*/
	// Check if Kokotap pod already exists, if not then create a new deployment
	foundKokotapPod := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: kokotap.Name, Namespace: kokotap.Namespace}, foundKokotapPod)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		runtime, err := r.getContainerRuntime()
		if err != nil {
			fmt.Printf("Error getting the runtime")
			return ctrl.Result{}, err
		}

		switch runtime {
		case "docker":
			// Generate deployment manifest for KokotapPod for Docker runtime
		case "cri-o":
			// Generate deployment manifest for KokotapPod for CRI-O runtime
		}

	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KokotapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Kokotap{}).
		Complete(r)
}

func (r *KokotapReconciler) getContainerRuntime() (string, error) {
	// Use in cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting in-cluster config: %v\n", err)
		return "", err
	}
	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating Kubernetes client: %v\n", err)
		return "", err
	}
	// Get a list of nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting nodes: %v\n", err)
		return "", err
	}

	// We assume all nodes use the same runtime
	nodeRuntime := nodes.Items[0].Status.NodeInfo.ContainerRuntimeVersion

	return strings.Split(nodeRuntime, ":")[0], nil

}
