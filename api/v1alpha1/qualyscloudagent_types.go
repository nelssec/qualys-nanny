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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QualysCloudAgentSpec defines the desired state of QualysCloudAgent.
type QualysCloudAgentSpec struct {
	// PlatformConfigRef references the QualysPlatformConfig to use
	// +kubebuilder:validation:Required
	PlatformConfigRef PlatformConfigReference `json:"platformConfigRef"`

	// Image defines the container image to use
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Config contains Qualys Cloud Agent configuration
	// +optional
	Config *CloudAgentConfig `json:"config,omitempty"`

	// Scheduling defines pod scheduling options
	// +optional
	Scheduling *SchedulingConfig `json:"scheduling,omitempty"`

	// Resources defines resource requirements for the agent container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// UpdateStrategy defines the DaemonSet update strategy
	// +optional
	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	// OpenShift defines OpenShift-specific settings
	// +optional
	OpenShift *OpenShiftConfig `json:"openshift,omitempty"`
}

// CloudAgentConfig defines Qualys Cloud Agent specific configuration
type CloudAgentConfig struct {
	// LogLevel sets the agent log verbosity (0-5)
	// 0=fatal, 1=error, 2=warning, 3=info, 4=debug, 5=trace
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=3
	LogLevel int `json:"logLevel,omitempty"`

	// LogFileDir is the directory for log files
	// +kubebuilder:validation:Pattern=`^(/[a-zA-Z0-9/_.-]+)?$`
	// +kubebuilder:validation:MaxLength=256
	// +optional
	LogFileDir string `json:"logFileDir,omitempty"`

	// LogDestType specifies log destination: "file" or "syslog"
	// +kubebuilder:validation:Enum=file;syslog
	// +optional
	LogDestType string `json:"logDestType,omitempty"`

	// LogCompression enables log compression on rollover
	// +optional
	LogCompression bool `json:"logCompression,omitempty"`

	// CmdMaxTimeOut is the maximum command execution time in seconds
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1800
	CmdMaxTimeOut int `json:"cmdMaxTimeOut,omitempty"`

	// ProcessPriority is the Linux niceness value (-20 to 19)
	// +kubebuilder:validation:Minimum=-20
	// +kubebuilder:validation:Maximum=19
	// +kubebuilder:default=0
	ProcessPriority int `json:"processPriority,omitempty"`

	// ScanDelayVM is the delay before VM scan in seconds (0-43200)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	ScanDelayVM *int `json:"scanDelayVM,omitempty"`

	// ScanDelayPC is the delay before PC scan in seconds (0-43200)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	ScanDelayPC *int `json:"scanDelayPC,omitempty"`

	// MaxRandomScanIntervalVM is the random interval for VM scans (0-43200)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	MaxRandomScanIntervalVM *int `json:"maxRandomScanIntervalVM,omitempty"`

	// MaxRandomScanIntervalPC is the random interval for PC scans (0-43200)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	MaxRandomScanIntervalPC *int `json:"maxRandomScanIntervalPC,omitempty"`

	// UseSudo enables running commands with sudo
	// +optional
	UseSudo bool `json:"useSudo,omitempty"`

	// SudoCommand is a custom privilege escalation command (e.g., pbrun)
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9/_.-]*$`
	// +kubebuilder:validation:MaxLength=128
	// +optional
	SudoCommand string `json:"sudoCommand,omitempty"`

	// User is the username for daemon execution
	// +optional
	User string `json:"user,omitempty"`

	// Group is the group for daemon execution
	// +optional
	Group string `json:"group,omitempty"`

	// UseAuditDispatcher enables FIM with auditd
	// +optional
	UseAuditDispatcher bool `json:"useAuditDispatcher,omitempty"`

	// HostIdSearchDir is the directory containing host ID file
	// +kubebuilder:validation:Pattern=`^(/[a-zA-Z0-9/_.-]+)?$`
	// +kubebuilder:validation:MaxLength=256
	// +optional
	HostIdSearchDir string `json:"hostIdSearchDir,omitempty"`
}

// QualysCloudAgentStatus defines the observed state of QualysCloudAgent.
type QualysCloudAgentStatus struct {
	// Conditions represent the latest available observations of the resource's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DesiredNumberScheduled is the total number of nodes that should be running the agent
	// +optional
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled,omitempty"`

	// CurrentNumberScheduled is the number of nodes running at least one agent pod
	// +optional
	CurrentNumberScheduled int32 `json:"currentNumberScheduled,omitempty"`

	// NumberReady is the number of nodes with ready agent pods
	// +optional
	NumberReady int32 `json:"numberReady,omitempty"`

	// NumberAvailable is the number of nodes with available agent pods
	// +optional
	NumberAvailable int32 `json:"numberAvailable,omitempty"`

	// UpdatedNumberScheduled is the number of nodes running updated agent pods
	// +optional
	UpdatedNumberScheduled int32 `json:"updatedNumberScheduled,omitempty"`

	// NumberMisscheduled is the number of nodes running agent pods that shouldn't be
	// +optional
	NumberMisscheduled int32 `json:"numberMisscheduled,omitempty"`

	// DaemonSetName is the name of the managed DaemonSet
	// +optional
	DaemonSetName string `json:"daemonSetName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.numberAvailable`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QualysCloudAgent is the Schema for the qualyscloudagents API.
// It manages the deployment of Qualys Cloud Agent as a DaemonSet.
type QualysCloudAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QualysCloudAgentSpec   `json:"spec,omitempty"`
	Status QualysCloudAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QualysCloudAgentList contains a list of QualysCloudAgent.
type QualysCloudAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QualysCloudAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QualysCloudAgent{}, &QualysCloudAgentList{})
}

// GetImage returns the image spec with defaults applied
func (s *QualysCloudAgentSpec) GetImage() ImageSpec {
	if s.Image != nil {
		img := *s.Image
		if img.Repository == "" {
			img.Repository = "nelssec/qualys-agent-bootstrapper"
		}
		if img.Tag == "" {
			img.Tag = "v2.1.0"
		}
		if img.PullPolicy == "" {
			img.PullPolicy = corev1.PullIfNotPresent
		}
		return img
	}
	return ImageSpec{
		Repository: "nelssec/qualys-agent-bootstrapper",
		Tag:        "v2.1.0",
		PullPolicy: corev1.PullIfNotPresent,
	}
}

// GetScheduling returns the scheduling config with defaults applied
func (s *QualysCloudAgentSpec) GetScheduling() SchedulingConfig {
	if s.Scheduling != nil {
		sched := *s.Scheduling
		if sched.NodeSelector == nil {
			sched.NodeSelector = map[string]string{
				"kubernetes.io/os": "linux",
			}
		}
		if len(sched.Tolerations) == 0 {
			sched.Tolerations = []corev1.Toleration{
				{
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoExecute,
				},
			}
		}
		if sched.PriorityClassName == "" {
			sched.PriorityClassName = "system-node-critical"
		}
		return sched
	}
	return SchedulingConfig{
		NodeSelector: map[string]string{
			"kubernetes.io/os": "linux",
		},
		Tolerations: []corev1.Toleration{
			{
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
			{
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
		},
		PriorityClassName: "system-node-critical",
	}
}

// GetResources returns the resource requirements with defaults applied
func (s *QualysCloudAgentSpec) GetResources() corev1.ResourceRequirements {
	if s.Resources != nil {
		return *s.Resources
	}
	return corev1.ResourceRequirements{}
}

// GetUpdateStrategy returns the update strategy with defaults applied
func (s *QualysCloudAgentSpec) GetUpdateStrategy() UpdateStrategyConfig {
	if s.UpdateStrategy != nil {
		return *s.UpdateStrategy
	}
	return UpdateStrategyConfig{
		Type: "RollingUpdate",
		RollingUpdate: &RollingUpdateConfig{
			MaxUnavailable: "25%",
		},
	}
}

// GetConfig returns the agent config with defaults applied
func (s *QualysCloudAgentSpec) GetConfig() CloudAgentConfig {
	if s.Config != nil {
		return *s.Config
	}
	return CloudAgentConfig{
		LogLevel:      3,
		CmdMaxTimeOut: 1800,
	}
}
