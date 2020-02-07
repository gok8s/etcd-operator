package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EtcdDumpPhase string

var (
	EtcdDumpRunning   EtcdDumpPhase = "Running"
	EtcdDumpCompleted EtcdDumpPhase = "Completed"
	EtcdDumpFailed    EtcdDumpPhase = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EtcdDumpSpec defines the desired state of EtcdDump
type EtcdDumpSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	ClusterReference string          `json:"clusterReference"`
	StorageProvider  StorageProvider `json:"storage"`
	Scheduler        string          `json:"scheduler,omitempty"`
}

type StorageProvider struct {
	S3    *S3StorageProvider    `json:"s3,omitempty"`
	Qiniu *QiniuStorageProvider `json:"qiniu,omitempty"`
}

type S3StorageProvider struct {
	Region            string                       `json:"region,omitempty"`
	Endpoint          string                       `json:"endpoint,omitempty"`
	Bucket            string                       `json:"bucket,omitempty"`
	ForcePathStyle    bool                         `json:"forcePathStyle,omitempty"`
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`
}

type QiniuStorageProvider struct {
	AccessKey string `json:"accesskey,omitempty"`
	SecretKey string `json:"secretkey,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	IO        string `json:"io,omitempty"`
	API       string `json:"api,omitempty"`
	UP        string `json:"up,omitempty"`
}

// EtcdDumpStatus defines the observed state of EtcdDump
type EtcdDumpStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Phase      EtcdDumpPhase       `json:"phase"`
	Conditions []EtcdDumpCondition `json:"conditions"`
}

type EtcdDumpCondition struct {
	Ready                 bool        `json:"ready"`
	Reason                string      `json:"reason,omitempty"`
	Message               string      `json:"message,omitempty"`
	LastedTranslationTime metav1.Time `json:"lastedTranslationTime"`
	Location              string      `json:"location,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdDump is the Schema for the etcddumps API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=etcddumps,scope=Namespaced
type EtcdDump struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdDumpSpec   `json:"spec,omitempty"`
	Status EtcdDumpStatus `json:"status,omitempty"`
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
