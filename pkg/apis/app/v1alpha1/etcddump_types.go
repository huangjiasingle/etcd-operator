package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EtcdDumpPhase string

var (
	EtcdDumpRunning   EtcdDumpPhase = "Running"
	EtcdDumpComplated EtcdDumpPhase = "Complated"
	EtcdDumpFailed    EtcdDumpPhase = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EtcdDumpSpec defines the desired state of EtcdDump
type EtcdDumpSpec struct {
	Scheduler        string          `json:"scheduler,omitempty"`
	ClusterReference string          `json:"clusterReference"`
	Storage          StorageProvider `json:"storgae"`
}

// EtcdDumpStatus defines the observed state of EtcdDump
type EtcdDumpStatus struct {
	Conditions []EtcdDumpCondition `json:"conditions,omitempty"`
	Phase      EtcdDumpPhase       `json:"phase"`
}

type EtcdDumpCondition struct {
	Ready                 bool        `json:"ready"`
	Location              string      `json:"location,omitempty"`
	Reason                string      `json:"reason,omitempty"`
	Message               string      `json:"message,omitempty"`
	LastedTranslationTime metav1.Time `json:"lastedTranslationTime"`
}

// StorageProvider defines the configuration for storing a Backup in a storage
// service.
type StorageProvider struct {
	S3 *S3StorageProvider `json:"s3,omitempty"`
	// TODO expensions another storage
	Qiniu *QiniuStorageProvider `json:"qiniu,omitempty"`
}

// S3StorageProvider represents an S3 compatible bucket for storing Backups.
type S3StorageProvider struct {
	// Region in which the S3 compatible bucket is located.
	Region string `json:"region,omitempty"`
	// Endpoint (hostname only or fully qualified URI) of S3 compatible
	// storage service.
	Endpoint string `json:"endpoint,omitempty"`
	// Bucket in which to store the Backup.
	Bucket string `json:"bucket,omitempty"`
	// ForcePathStyle when set to true forces the request to use path-style
	// addressing, i.e., `http://s3.amazonaws.com/BUCKET/KEY`. By default,
	// the S3 client will use virtual hosted bucket addressing when possible
	// (`http://BUCKET.s3.amazonaws.com/KEY`).
	ForcePathStyle bool `json:"forcePathStyle,omitempty"`
	// CredentialsSecret is a reference to the Secret containing the
	// credentials authenticating with the S3 compatible storage service.
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
}

// QiniuStorageProvider represents an qiniu compatible bucket for storing Backups.
type QiniuStorageProvider struct {
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	IO        string `json:"io,omitempty"`
	API       string `json:"api,omitempty"`
	UP        string `json:"up,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdDump is the Schema for the etcddumps API
// +k8s:openapi-gen=true
type EtcdDump struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EtcdDumpSpec   `json:"spec,omitempty"`
	Status            EtcdDumpStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdDumpList contains a list of EtcdDump
type EtcdDumpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdDump `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdDump{}, &EtcdDumpList{})
}
