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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type QualysPlatformConfigSpec struct {
	// +kubebuilder:validation:Required
	Platform PlatformSettings `json:"platform"`

	// +kubebuilder:validation:Required
	Credentials CredentialsConfig `json:"credentials"`
}

type PlatformSettings struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https://.*`
	ServerUri string `json:"serverUri"`

	// +kubebuilder:validation:Pattern=`^https://.*`
	GatewayUrl string `json:"gatewayUrl,omitempty"`

	Proxy *ProxyConfig `json:"proxy,omitempty"`
}

type CredentialsConfig struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=secret;externalSecret
	SourceType CredentialSourceType `json:"sourceType"`

	SecretRef *SecretReference `json:"secretRef,omitempty"`

	ExternalSecretRef *ExternalSecretReference `json:"externalSecretRef,omitempty"`
}

type QualysPlatformConfigStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	CredentialSource string `json:"credentialSource,omitempty"`

	LastValidated *metav1.Time `json:"lastValidated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Server",type=string,JSONPath=`.spec.platform.serverUri`
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.credentials.sourceType`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="CredentialsReady")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type QualysPlatformConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QualysPlatformConfigSpec   `json:"spec,omitempty"`
	Status QualysPlatformConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type QualysPlatformConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QualysPlatformConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QualysPlatformConfig{}, &QualysPlatformConfigList{})
}
