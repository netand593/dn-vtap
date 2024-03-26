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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MirrorType defines the type of traffic to be mirrored
// +kubebuilder:validation:Enum=both;ingress;egress
type MirrorType string

const (
	BothMirrorType    MirrorType = "both"
	IngressMirrorType MirrorType = "ingress"
	EgressMirrorType  MirrorType = "egress"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KokotapSpec defines the desired state of Kokotap
type KokotapSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Pod's name in which the packet capture should be done
	PodName string `json:"podName"`
	// IP address of the host which will receive the captured packets
	TargetIP string `json:"destIp"`
	// VXLAN ID
	VxLANID int32 `json:"vxlanID"`
	// Namespace of the pod
	Namespace string `json:"namespace"`
	// Type of mirror traffic
	MirrorType string `json:"mirrorType"`
	// Pod Interface to do the tapping (mirror traffic)
	PodInterface string `json:"podInterface"`
	// Image to be used for the kokotap container
	Image string `json:"image"`

	/* TODO
	Check if it is needed to add kubeconfig file to the spec
	*/

	// +kubebuilder:validation:Enum=UDP;TCP

}

// KokotapStatus defines the observed state of Kokotap
type KokotapStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Kokotap is the Schema for the kokotaps API
type Kokotap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KokotapSpec   `json:"spec,omitempty"`
	Status KokotapStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KokotapList contains a list of Kokotap
type KokotapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kokotap `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kokotap{}, &KokotapList{})
}
