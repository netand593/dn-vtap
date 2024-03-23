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
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	// "sigs.k8s.io/cluster-api/util/patch"
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
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=pods/finalizers,verbs=update
//+kubebuilder:scaffold:rbac:groups=networking.dn-lab.io,resources=kokotaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:scaffold:rbac:groups=networking.dn-lab.io,resources=kokotaps/status,verbs=get;update;patch
//+kubebuilder:scaffold:rbac:groups=networking.dn-lab.io,resources=kokotaps/finalizers,verbs=update
//+kubebuilder:scaffold:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

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
	logger.Info("Reconciling Kokotap")

	// Fetch the Kokotap instance
	kokotap := &networkingv1alpha1.Kokotap{}
	if err := r.Get(ctx, req.NamespacedName, kokotap); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Kokotap resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get Kokotap object")
		return ctrl.Result{}, err
	}

	// Check if the Kokotap instance is marked to be deleted

	if kokotap.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(kokotap, FinalizerName) {
			logger.Info("Kokotap Marked for deletion, deleting")
			return r.ReconcileDelete(ctx, kokotap)
		}
	}

	// Reconcile the Kokotap instance

	return r.ReconcileNormal(ctx, kokotap)

}

func (r *KokotapReconciler) CreateKokotapPod(ctx context.Context, kokotap *networkingv1alpha1.Kokotap) (corev1.Pod, error) {

	logger := log.FromContext(ctx)
	var pod corev1.Pod
	var tappedpod corev1.Pod
	tappedPodName := "kokotapped-" + kokotap.Spec.PodName

	// Call GetPodInfo to get the pod info

	podInfo := r.GetPodInfo(kokotap, ctx)

	// Get tappedpod and update labels

	err := r.Get(ctx, types.NamespacedName{Name: kokotap.Spec.PodName, Namespace: kokotap.Spec.Namespace}, &tappedpod)
	if err != nil {
		logger.Error(err, "Failed to get Pod", "Pod.Namespace", kokotap.Spec.Namespace, "Pod.Name", kokotap.Spec.PodName)
	}

	// Add the label to the pod

	if tappedpod.Labels == nil {
		tappedpod.Labels = make(map[string]string)
	}
	tappedpod.Labels["dn-vtap"] = "tapped"

	// Update the pod

	if err := r.Update(ctx, &tappedpod); err != nil {
		logger.Error(err, "Failed to update Pod", "Pod.Namespace", kokotap.Spec.Namespace, "Pod.Name", kokotap.Spec.PodName)
		return pod, err
	}
	logger.Info("Updated Pod successfully", "Pod.Namespace", kokotap.Spec.Namespace, "Pod.Name", kokotap.Spec.PodName)

	// Create the pod running kokotap_pod binary

	pod = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tappedPodName,
			Namespace: kokotap.Spec.Namespace,
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
					Name:            "kokotap-network-tap",
					Image:           kokotap.Spec.Image,
					ImagePullPolicy: "Always",
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
					Command: []string{"/bin/kokotap_pod"},
					Args: []string{
						"--procprefix=/host",
						"mode",
						"sender",
						"--containerid=" + podInfo.containerID,
						"--mirrortype=" + kokotap.Spec.MirrorType,
						"--ifname=mirror",
						"--mirrorif=" + kokotap.Spec.PodInterface,
						"--vxlan-egressip=" + podInfo.nodeIP,
						"--vxlan-ip=" + kokotap.Spec.TargetIP,
						"--vxlan-id=" + strconv.Itoa(int(kokotap.Spec.VxLANID)),
						"--vxlan-port=4789",
					},
				},
			},
		},
	}

	// Create the pod

	if err := r.Create(ctx, &pod); err != nil {
		logger.Error(err, "Failed to create Pod", "Pod.Namespace", kokotap.Spec.Namespace, "Pod.Name", tappedPodName)
		return pod, err
	}
	logger.Info("Created Pod successfully", "Pod.Namespace", kokotap.Spec.Namespace, "Pod.Name", tappedPodName)
	return pod, nil
}

// ReconcileNormal is the function that will be called when the resource is not being deleted or updated
func (r *KokotapReconciler) ReconcileNormal(ctx context.Context, kokotap *networkingv1alpha1.Kokotap) (ctrl.Result, error) {

	logger := log.FromContext(ctx)
	kokotapPod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: "kokotapped-" + kokotap.Spec.PodName, Namespace: kokotap.Spec.Namespace}, kokotapPod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Kokotap Pod not found. Creating one")
			_, err = r.CreateKokotapPod(ctx, kokotap)
			if err != nil {
				logger.Error(err, "Failed to create Kokotap Pod")
				return ctrl.Result{}, err
			}
			logger.Info("Created Kokotap Pod successfully")
			// Add finalizer to the Kokotap Custom Resource becasue the Kokotap Pod has been created
			if !controllerutil.ContainsFinalizer(kokotap, FinalizerName) {
				controllerutil.AddFinalizer(kokotap, FinalizerName)
				err = r.Update(ctx, kokotap)
				if err != nil {
					logger.Error(err, "Failed to add finalizer to Kokotap")
					return ctrl.Result{}, err
				}
				logger.Info("Added finalizer to Kokotap")
			}
			return ctrl.Result{}, nil
		}
		// Todo: Handle other errors
		logger.Error(err, "Failed to fetch Kokotap Pod")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// ReconcileDelete deletes the kokotap_pod and removes the label from the tapped pod
func (r *KokotapReconciler) ReconcileDelete(ctx context.Context, kokotap *networkingv1alpha1.Kokotap) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// Delete the kokotap_pod and remove label from the tapped pod

	podname := "kokotapped-" + kokotap.Spec.PodName
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podname, Namespace: kokotap.Spec.Namespace}, pod)
	if err != nil {
		logger.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	// Check kokotap status to handle deletion
	if pod.Status.Phase == corev1.PodRunning {
		if err := r.Delete(ctx, pod); err != nil {
			logger.Error(err, "Failed to delete Pod")
			return ctrl.Result{}, err
		}
		logger.Info("Deleted Pod successfully")
	}

	// Remove the label from the tapped pod
	tappedpod := &corev1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: kokotap.Spec.PodName, Namespace: kokotap.Spec.Namespace}, tappedpod)
	if err != nil {
		logger.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}
	tappedpod.Labels["dn-vtap"] = "not-tapped"
	if err := r.Update(ctx, tappedpod); err != nil {
		logger.Error(err, "Failed to update Pod")
		return ctrl.Result{}, err
	}

	// Remove the finalizer from the Kokotap CR

	controllerutil.RemoveFinalizer(kokotap, FinalizerName)
	if err := r.Update(ctx, kokotap); err != nil {
		logger.Error(err, "Failed to remove finalizer from Kokotap")
		return ctrl.Result{}, err
	}
	logger.Info("Removed finalizer from Kokotap")

	return ctrl.Result{}, nil
}

func (r *KokotapReconciler) GetPodInfo(kokotap *networkingv1alpha1.Kokotap, ctx context.Context) *PodInfo {

	logger := log.FromContext(ctx)

	// Get the pod info

	TappedPodName := kokotap.Spec.PodName
	TappedNamespace := kokotap.Spec.Namespace

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
