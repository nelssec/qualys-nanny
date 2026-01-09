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

// +kubebuilder:validation:Enum=auto;bootstrapper;coreos
type DeploymentMode string

const (
	DeploymentModeAuto         DeploymentMode = "auto"
	DeploymentModeBootstrapper DeploymentMode = "bootstrapper"
	DeploymentModeCoreOS       DeploymentMode = "coreos"
)

type QualysCloudAgentSpec struct {
	// +kubebuilder:validation:Required
	PlatformConfigRef PlatformConfigReference `json:"platformConfigRef"`

	// +kubebuilder:default=auto
	// +optional
	DeploymentMode DeploymentMode `json:"deploymentMode,omitempty"`

	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// +optional
	Config *CloudAgentConfig `json:"config,omitempty"`

	// +optional
	CoreOSConfig *CoreOSConfig `json:"coreosConfig,omitempty"`

	// +optional
	Scheduling *SchedulingConfig `json:"scheduling,omitempty"`

	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	// +optional
	OpenShift *OpenShiftConfig `json:"openshift,omitempty"`
}

type CoreOSConfig struct {
	// +kubebuilder:validation:Pattern=`^[0-9]+m?$`
	// +kubebuilder:default="200m"
	// +optional
	CPULimit string `json:"cpuLimit,omitempty"`

	// +kubebuilder:validation:Enum=AWS;AZURE;GCP;IBM;ALIBABA;ORACLE;NONE;AUTO
	// +kubebuilder:default=AUTO
	// +optional
	ProviderName string `json:"providerName,omitempty"`
}

type CloudAgentConfig struct {
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=3
	LogLevel int `json:"logLevel,omitempty"`

	// +kubebuilder:validation:Pattern=`^(/[a-zA-Z0-9/_.-]+)?$`
	// +kubebuilder:validation:MaxLength=256
	// +optional
	LogFileDir string `json:"logFileDir,omitempty"`

	// +kubebuilder:validation:Enum=file;syslog
	// +optional
	LogDestType string `json:"logDestType,omitempty"`

	// +optional
	LogCompression bool `json:"logCompression,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1800
	CmdMaxTimeOut int `json:"cmdMaxTimeOut,omitempty"`

	// +kubebuilder:validation:Minimum=-20
	// +kubebuilder:validation:Maximum=19
	// +kubebuilder:default=0
	ProcessPriority int `json:"processPriority,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	ScanDelayVM *int `json:"scanDelayVM,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	ScanDelayPC *int `json:"scanDelayPC,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	MaxRandomScanIntervalVM *int `json:"maxRandomScanIntervalVM,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=43200
	// +optional
	MaxRandomScanIntervalPC *int `json:"maxRandomScanIntervalPC,omitempty"`

	// +optional
	UseSudo bool `json:"useSudo,omitempty"`

	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9/_.-]*$`
	// +kubebuilder:validation:MaxLength=128
	// +optional
	SudoCommand string `json:"sudoCommand,omitempty"`

	// +optional
	User string `json:"user,omitempty"`

	// +optional
	Group string `json:"group,omitempty"`

	// +optional
	UseAuditDispatcher bool `json:"useAuditDispatcher,omitempty"`

	// +kubebuilder:validation:Pattern=`^(/[a-zA-Z0-9/_.-]+)?$`
	// +kubebuilder:validation:MaxLength=256
	// +optional
	HostIdSearchDir string `json:"hostIdSearchDir,omitempty"`
}

type QualysCloudAgentStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	DeploymentMode string `json:"deploymentMode,omitempty"`

	// +optional
	DetectedOS string `json:"detectedOS,omitempty"`

	// +optional
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled,omitempty"`

	// +optional
	CurrentNumberScheduled int32 `json:"currentNumberScheduled,omitempty"`

	// +optional
	NumberReady int32 `json:"numberReady,omitempty"`

	// +optional
	NumberAvailable int32 `json:"numberAvailable,omitempty"`

	// +optional
	UpdatedNumberScheduled int32 `json:"updatedNumberScheduled,omitempty"`

	// +optional
	NumberMisscheduled int32 `json:"numberMisscheduled,omitempty"`

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
type QualysCloudAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QualysCloudAgentSpec   `json:"spec,omitempty"`
	Status QualysCloudAgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type QualysCloudAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QualysCloudAgent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QualysCloudAgent{}, &QualysCloudAgentList{})
}

func (s *QualysCloudAgentSpec) GetImage(effectiveMode DeploymentMode) ImageSpec {
	defaultRepo := "nelssec/qualys-agent-bootstrapper"
	defaultTag := "v2.1.0"

	if effectiveMode == DeploymentModeCoreOS {
		defaultRepo = "qualys/qagent-rhcos"
		defaultTag = "latest"
	}

	if s.Image != nil {
		img := *s.Image
		if img.Repository == "" {
			img.Repository = defaultRepo
		}
		if img.Tag == "" {
			img.Tag = defaultTag
		}
		if img.PullPolicy == "" {
			img.PullPolicy = corev1.PullIfNotPresent
		}
		return img
	}
	return ImageSpec{
		Repository: defaultRepo,
		Tag:        defaultTag,
		PullPolicy: corev1.PullIfNotPresent,
	}
}

func (s *QualysCloudAgentSpec) GetCoreOSConfig() CoreOSConfig {
	if s.CoreOSConfig != nil {
		cfg := *s.CoreOSConfig
		if cfg.CPULimit == "" {
			cfg.CPULimit = "200m"
		}
		if cfg.ProviderName == "" {
			cfg.ProviderName = "AUTO"
		}
		return cfg
	}
	return CoreOSConfig{
		CPULimit:     "200m",
		ProviderName: "AUTO",
	}
}

func (s *QualysCloudAgentSpec) GetDeploymentMode() DeploymentMode {
	if s.DeploymentMode == "" {
		return DeploymentModeAuto
	}
	return s.DeploymentMode
}

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

func (s *QualysCloudAgentSpec) GetResources() corev1.ResourceRequirements {
	if s.Resources != nil {
		return *s.Resources
	}
	return corev1.ResourceRequirements{}
}

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

func (s *QualysCloudAgentSpec) GetConfig() CloudAgentConfig {
	if s.Config != nil {
		return *s.Config
	}
	return CloudAgentConfig{
		LogLevel:      3,
		CmdMaxTimeOut: 1800,
	}
}
