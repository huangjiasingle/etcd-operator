package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EtcdRestorePhase string

var (
	EtcdRestoreRunning   EtcdRestorePhase = "Running"
	EtcdRestoreComplated EtcdRestorePhase = "Complated"
	EtcdRestoreFailed    EtcdRestorePhase = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EtcdRestoreSpec defines the desired state of EtcdRestore
type EtcdRestoreSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	ClusterReference string `json:"clusterReference"`
	DataURl          string `json:"dataURL"`
}

// EtcdRestoreStatus defines the observed state of EtcdRestore
type EtcdRestoreStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Conditions []EtcdRestoreCondition `json:"conditions"`
	Phase      EtcdRestorePhase       `json:"phase"`
}

type EtcdRestoreCondition struct {
	Ready                 bool        `json:"ready"`
	Reason                string      `json:"reason,omitempty"`
	Message               string      `json:"message,omitempty"`
	LastedTranslationTime metav1.Time `json:"lastedTranslationTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdRestore is the Schema for the etcdrestores API
// +k8s:openapi-gen=true
type EtcdRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdRestoreSpec   `json:"spec,omitempty"`
	Status EtcdRestoreStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdRestoreList contains a list of EtcdRestore
type EtcdRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdRestore{}, &EtcdRestoreList{})
}
