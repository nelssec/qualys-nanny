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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=secret;externalSecret
type CredentialSourceType string

const (
	CredentialSourceSecret         CredentialSourceType = "secret"
	CredentialSourceExternalSecret CredentialSourceType = "externalSecret"
)

type SecretReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

type ExternalSecretReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
	// +kubebuilder:validation:Required
	SecretStoreRef SecretStoreReference `json:"secretStoreRef"`
	// +kubebuilder:validation:Required
	KeyMappings ExternalSecretKeyMappings `json:"keyMappings"`
}

type SecretStoreReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=SecretStore;ClusterSecretStore
	// +kubebuilder:default=ClusterSecretStore
	Kind string `json:"kind,omitempty"`
}

type ExternalSecretKeyMappings struct {
	// +kubebuilder:validation:Required
	ActivationId string `json:"activationId"`
	// +kubebuilder:validation:Required
	CustomerId string `json:"customerId"`
}

type ProxyConfig struct {
	QualysHttpsProxy string `json:"qualysHttpsProxy,omitempty"`
	HttpsProxy       string `json:"httpsProxy,omitempty"`
	// +kubebuilder:validation:Enum=sequential;random
	ProxyOrder    string `json:"proxyOrder,omitempty"`
	ProxyFailOpen bool   `json:"proxyFailOpen,omitempty"`
	CACertBundle  string `json:"caCertBundle,omitempty"`
}

type ImageSpec struct {
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`
	Tag        string `json:"tag,omitempty"`
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	PullPolicy  corev1.PullPolicy             `json:"pullPolicy,omitempty"`
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`
}

type SchedulingConfig struct {
	NodeSelector map[string]string   `json:"nodeSelector,omitempty"`
	Tolerations  []corev1.Toleration `json:"tolerations,omitempty"`
	Affinity     *corev1.Affinity    `json:"affinity,omitempty"`
	// +kubebuilder:default=system-node-critical
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

type UpdateStrategyConfig struct {
	// +kubebuilder:validation:Enum=RollingUpdate;OnDelete
	// +kubebuilder:default=RollingUpdate
	Type          string               `json:"type,omitempty"`
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

type RollingUpdateConfig struct {
	// +kubebuilder:default="25%"
	MaxUnavailable string `json:"maxUnavailable,omitempty"`
}

type OpenShiftConfig struct {
	SCC                SCCConfig `json:"scc,omitempty"`
	ServiceAccountName string    `json:"serviceAccountName,omitempty"`
}

type SCCConfig struct {
	// +kubebuilder:default=true
	Create bool   `json:"create,omitempty"`
	Name   string `json:"name,omitempty"`
}

type PlatformConfigReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

const (
	ConditionTypeAvailable         string = "Available"
	ConditionTypeProgressing       string = "Progressing"
	ConditionTypeDegraded          string = "Degraded"
	ConditionTypeCredentialsReady  string = "CredentialsReady"
	ConditionTypePlatformReachable string = "PlatformReachable"
)

func SetCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	for i, c := range *conditions {
		if c.Type == condition.Type {
			if c.Status != condition.Status || c.Reason != condition.Reason || c.Message != condition.Message {
				condition.LastTransitionTime = metav1.Now()
				(*conditions)[i] = condition
			}
			return
		}
	}
	condition.LastTransitionTime = metav1.Now()
	*conditions = append(*conditions, condition)
}

func GetCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
