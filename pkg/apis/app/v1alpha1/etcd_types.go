package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InitClusterType string

var (
	StaticInitClusterType InitClusterType = "static"
	DNSInitClusterType    InitClusterType = "dns-discovery"
	ETCDInitClusterType   InitClusterType = "etcd-discovery"
)

// EtcdSpec defines the desired state of Etcd
type EtcdSpec struct {
	// replicas etcd cluster size
	Replicas       *int32                      `json:"replicas"`
	Image          string                      `json:"image"`
	Cluster        bool                        `json:"cluster"`
	Insecure       bool                        `json:"insecure"`
	Storage        int32                       `json:"storage"`
	InitCluserType InitClusterType             `json:"initClusterType"`
	Resources      corev1.ResourceRequirements `json:"resources"`
}

// EtcdStatus defines the observed state of Etcd
type EtcdStatus struct {
	appsv1.StatefulSetStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Etcd is the Schema for the etcds API
// +k8s:openapi-gen=true
type Etcd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdSpec   `json:"spec,omitempty"`
	Status EtcdStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdList contains a list of Etcd
type EtcdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Etcd `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Etcd{}, &EtcdList{})
}
