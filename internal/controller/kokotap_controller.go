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

	errors "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1alpha1 "github.com/netand593/dn-vtap/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// KokotapReconciler reconciles a Kokotap object
type KokotapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	// FinalizerName is the name of the finalizer added to resources for this controller
	FinalizerName = "kokotap.networking.dn-lab.io"

	//
)

// Pod Info that will be fetched, given the PodName
type PodInfo struct {
	containerID string
	nodeName    string
	nodeIP      string
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
func (r *KokotapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, errResult error) {
	logger := log.FromContext(ctx)

	kokotapper := &networkingv1alpha1.Kokotap{}
	if err := r.Get(ctx, req.NamespacedName, kokotapper); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Kokotap resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to fetch kokotap")
		return ctrl.Result{}, err
	}

	// Initialize the patch helper

	helper, err := patch.NewHelper(kokotapper, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create patch helper")
	}
	defer func() {
		// patch the resource stored in the API server if something changed.
		if err := helper.Patch(ctx, kokotapper); err != nil {
			errResult = err
		}
	}()
	// Check if the resource is being deleted

	if kokotapper.DeletionTimestamp.IsZero() {
		return r.ReconcileNormal(ctx, req, kokotapper)
	}

	// kokotapper has been marked for delete

	return r.ReconcileDelete(ctx, req, kokotapper)
}

func (r *KokotapReconciler) GetKokotapper(ctx context.Context, req ctrl.Request) (*networkingv1alpha1.Kokotap, error) {
	logger := log.FromContext(ctx)
	kokotapper := &networkingv1alpha1.Kokotap{}
	if err := r.Get(ctx, req.NamespacedName, kokotapper); err != nil {
		logger.Error(err, "Failed to fetch Kokotap from API Server")
		return nil, err
	}
	return kokotapper, nil
}

func (r *KokotapReconciler) CreateKokotapper(ctx context.Context, req ctrl.Request, vtap *networkingv1alpha1.Kokotap) (corev1.Pod, error) {

	logger := log.FromContext(ctx)
	var pod corev1.Pod
	var tappedpod corev1.Pod
	tappedPodName := "kokotapped_" + vtap.Spec.PodName

	// Call GetPodInfo to get the pod info

	podInfo := r.GetPodInfo(vtap, ctx)

	// Get tappedpod and update labels

	err := r.Get(ctx, types.NamespacedName{Name: vtap.Spec.PodName, Namespace: vtap.Namespace}, &tappedpod)
	if err != nil {
		logger.Error(err, "Failed to get Pod", "Pod.Namespace", vtap.Namespace, "Pod.Name", vtap.Spec.PodName)
	}

	// Add the label to the pod

	if tappedpod.Labels == nil {
		tappedpod.Labels = make(map[string]string)
	}
	tappedpod.Labels["dn-vtap"] = "tapped"

	// Update the pod

	if err := r.Update(ctx, &tappedpod); err != nil {
		logger.Error(err, "Failed to update Pod", "Pod.Namespace", vtap.Namespace, "Pod.Name", vtap.Spec.PodName)
		return pod, err
	}
	logger.Info("Updated Pod successfully", "Pod.Namespace", vtap.Namespace, "Pod.Name", vtap.Spec.PodName)

	// Create the pod running kokotap_pod binary

	pod = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tappedPodName,
			Namespace: vtap.Namespace,
			Labels: map[string]string{
				"dn-vtap": "tapped",
			},
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			NodeName:    podInfo.nodeName,
			Volumes: []corev1.Volume{
				{
					Name: "proc",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/proc",
						},
					},
				},
				{
					Name: "var-crio",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/run/crio/crio.sock",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "kokotap-network-tap",
					Image: vtap.Spec.Image,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &[]bool{true}[0],
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "proc",
							MountPath: "/host/proc",
						},
						{
							Name:      "var-crio",
							MountPath: "/var/run/crio/crio.sock",
						},
					},
					Command: []string{"/bin/kokotap"},
					Args: []string{
						"--procprefix=/hostproc",
						"mode",
						"sender",
						"--containerid=" + podInfo.containerID,
						"--mirrortype=" + vtap.Spec.MirrorType,
						"--ifname= mirror",
						"--mirrorif=" + vtap.Spec.PodInterface,
						"--pod=" + vtap.Spec.PodName,
						"--namespace=" + vtap.Namespace,
						"--vxlan-egressip=" + podInfo.nodeIP,
						"--vxlan-ip=" + vtap.Spec.TargetIP,
						"--vxlan-id=" + vtap.Spec.VxLANID,
						"--vxlan-port=4789",
					},
				},
			},
		},
	}

	// Create the pod

	if err := r.Create(ctx, &pod); err != nil {
		logger.Error(err, "Failed to create Pod", "Pod.Namespace", vtap.Namespace, "Pod.Name", tappedPodName)
		return pod, err
	}
	logger.Info("Created Pod successfully", "Pod.Namespace", vtap.Namespace, "Pod.Name", tappedPodName)
	return pod, nil
}

// ReconcileNormal is the function that will be called when the resource is not being deleted or updated
func (r *KokotapReconciler) ReconcileNormal(ctx context.Context, req ctrl.Request, vtap *networkingv1alpha1.Kokotap) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// Add the finalizer to the resource if it does not exist

	if !controllerutil.ContainsFinalizer(vtap, FinalizerName) {
		controllerutil.AddFinalizer(vtap, FinalizerName)
		if err := r.Update(ctx, vtap); err != nil {
			logger.Error(err, "Failed to add finalizer to Kokotap", "Kokotap.Namespace", vtap.Namespace, "Kokotap.Name", vtap.Name)
			return ctrl.Result{}, err
		}
		logger.Info("Added finalizer to Kokotap", "Kokotap.Namespace", vtap.Namespace, "Kokotap.Name", vtap.Name)
	}

	var pod corev1.Pod

	// Check if the pod exists

	err := r.Get(ctx, types.NamespacedName{Name: vtap.Spec.PodName, Namespace: vtap.Spec.Namespace}, &pod)

	if err != nil && client.IgnoreNotFound(err) != nil {
		// Pod does not exist, create it
		pod, err = r.CreateKokotapper(ctx, req, vtap)
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.Error(err, "Failed to create Pod", "Pod.Namespace", vtap.Namespace, "Pod.Name", vtap.Spec.PodName)
	}
	return ctrl.Result{}, nil

}

func (r *KokotapReconciler) ReconcileDelete(ctx context.Context, req ctrl.Request, vtap *networkingv1alpha1.Kokotap) (ctrl.Result, error) {
	// Delete the resources

	logger := log.FromContext(ctx)

	// Remove the finalizer from the resource

	if controllerutil.ContainsFinalizer(vtap, FinalizerName) {
		controllerutil.RemoveFinalizer(vtap, FinalizerName)
		if err := r.Update(ctx, vtap); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Delete the kokotapper

	podname := "kokotapped_" + vtap.Spec.PodName
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podname, Namespace: vtap.Namespace}, pod)
	if err != nil {
		logger.Error(err, "Failed to get Pod")
	}

	// Check kokotapper status to handle deletion

	if pod.Status.Phase == corev1.PodRunning {
		if err := r.Delete(ctx, pod); err != nil {
			logger.Error(err, "Failed to delete Pod")
		}
	}

	// Remove the label from the tapped pod

	tappedpod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: vtap.Spec.PodName, Namespace: vtap.Namespace}, tappedpod)
	if err != nil {
		logger.Error(err, "Failed to get Pod")
	}
	tappedpod.Labels["dn-vtap"] = "not-tapped"
	if err := r.Update(ctx, tappedpod); err != nil {
		logger.Error(err, "Failed to update Pod")
	}

	return ctrl.Result{}, nil
}

func (r *KokotapReconciler) GetPodInfo(vtap *networkingv1alpha1.Kokotap, ctx context.Context) *PodInfo {

	logger := log.FromContext(ctx)

	// Get the pod info

	TappedPodName := vtap.Spec.PodName
	TappedNamespace := vtap.Spec.Namespace

	pod := &corev1.Pod{}

	err := r.Get(ctx, types.NamespacedName{Name: TappedPodName, Namespace: TappedNamespace}, pod)
	if err != nil {
		logger.Error(err, "Failed to get Pod", "Pod.Namespace", TappedNamespace, "Pod.Name", TappedPodName)
	}

	// Get the containerID
	containerID := pod.Status.ContainerStatuses[0].ContainerID
	// Get the nodeName
	nodeName := pod.Spec.NodeName
	// Get the nodeIP
	nodeIP := pod.Status.HostIP

	return &PodInfo{
		containerID: containerID,
		nodeName:    nodeName,
		nodeIP:      nodeIP,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *KokotapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Kokotap{}).
		Complete(r)
}
