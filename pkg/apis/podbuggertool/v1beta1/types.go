package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Podbuggertool specification
type Podbuggertool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodbuggertoolSpec   `json:"spec"`
	Status PodbuggertoolStatus `json:"status"`
}

// PodbuggertoolSpec
type PodbuggertoolSpec struct {
	Label string `json:"label"`
	Image string `json:"image"`
}

// PodbuggertoolStatus is the status for a Podbuggertool resource
type PodbuggertoolStatus struct {
	Installed int32 `json:"installed"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodbuggertoolList is a list of Podbuggertool resources
type PodbuggertoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Podbuggertool `json:"items"`
}
