/*
Copyright 2026.

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

// QualysPlatformConfigSpec defines the desired state of QualysPlatformConfig.
type QualysPlatformConfigSpec struct {
	// Platform contains Qualys platform connection settings
	// +kubebuilder:validation:Required
	Platform PlatformSettings `json:"platform"`

	// Credentials contains credential source configuration
	// +kubebuilder:validation:Required
	Credentials CredentialsConfig `json:"credentials"`
}

// PlatformSettings defines Qualys platform connection parameters
type PlatformSettings struct {
	// ServerUri is the Qualys platform URL
	// Example: https://qagpublic.qg2.apps.qualys.com/CloudAgent/
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	ServerUri string `json:"serverUri"`

	// Proxy contains optional proxy configuration
	// +optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`
}

// CredentialsConfig defines how credentials are sourced
type CredentialsConfig struct {
	// SourceType specifies the credential source type
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=secret;externalSecret
	SourceType CredentialSourceType `json:"sourceType"`

	// SecretRef references a Kubernetes Secret containing credentials
	// Required when sourceType is "secret"
	// The Secret must contain ACTIVATION_ID and CUSTOMER_ID keys
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// ExternalSecretRef references an External Secrets configuration
	// Required when sourceType is "externalSecret"
	// +optional
	ExternalSecretRef *ExternalSecretReference `json:"externalSecretRef,omitempty"`
}

// QualysPlatformConfigStatus defines the observed state of QualysPlatformConfig.
type QualysPlatformConfigStatus struct {
	// Conditions represent the latest available observations of the resource's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CredentialSource describes the resolved credential source
	// +optional
	CredentialSource string `json:"credentialSource,omitempty"`

	// LastValidated is the timestamp of the last successful credential validation
	// +optional
	LastValidated *metav1.Time `json:"lastValidated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.platform.serverUri`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.credentials.sourceType`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="CredentialsReady")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QualysPlatformConfig is the Schema for the qualysplatformconfigs API.
// It provides cluster-wide Qualys platform configuration including credentials.
type QualysPlatformConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QualysPlatformConfigSpec   `json:"spec,omitempty"`
	Status QualysPlatformConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QualysPlatformConfigList contains a list of QualysPlatformConfig.
type QualysPlatformConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QualysPlatformConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QualysPlatformConfig{}, &QualysPlatformConfigList{})
}
