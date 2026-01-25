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

// +kubebuilder:validation:Enum=cri-o;containerd;docker
type ContainerRuntimeType string

const (
	ContainerRuntimeCRIO       ContainerRuntimeType = "cri-o"
	ContainerRuntimeContainerd ContainerRuntimeType = "containerd"
	ContainerRuntimeDocker     ContainerRuntimeType = "docker"
)

// +kubebuilder:validation:Enum=general;registry;cicd
type ContainerSensorMode string

const (
	ContainerSensorModeGeneral  ContainerSensorMode = "general"
	ContainerSensorModeRegistry ContainerSensorMode = "registry"
	ContainerSensorModeCICD     ContainerSensorMode = "cicd"
)

// +kubebuilder:validation:Enum=unprivileged;minimal;standard;privileged
type PrivilegeMode string

const (
	PrivilegeModeUnprivileged PrivilegeMode = "unprivileged"
	PrivilegeModeMinimal      PrivilegeMode = "minimal"
	PrivilegeModeStandard     PrivilegeMode = "standard"
	PrivilegeModePrivileged   PrivilegeMode = "privileged"
)

// +kubebuilder:validation:Enum=DynamicWithStaticScanningAsFallback;DynamicScanningOnly;StaticScanningOnly
type ScanningPolicy string

const (
	ScanningPolicyDynamicWithStaticFallback ScanningPolicy = "DynamicWithStaticScanningAsFallback"
	ScanningPolicyDynamicOnly               ScanningPolicy = "DynamicScanningOnly"
	ScanningPolicyStaticOnly                ScanningPolicy = "StaticScanningOnly"
)

type QualysContainerSecuritySpec struct {
	// +kubebuilder:validation:Required
	PlatformConfigRef PlatformConfigReference `json:"platformConfigRef"`

	ContainerSensor *ContainerSensorConfig `json:"containerSensor,omitempty"`

	ClusterSensor *ClusterSensorConfig `json:"clusterSensor,omitempty"`

	AdmissionController *AdmissionControllerConfig `json:"admissionController,omitempty"`

	RuntimeSensor *RuntimeSensorConfig `json:"runtimeSensor,omitempty"`

	ContainerRuntime *ContainerRuntimeConfig `json:"containerRuntime,omitempty"`

	Scheduling *SchedulingConfig `json:"scheduling,omitempty"`

	OpenShift *OpenShiftConfig `json:"openshift,omitempty"`
}

type ContainerSensorConfig struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Enum=general;registry;cicd
	// +kubebuilder:default=general
	Mode ContainerSensorMode `json:"mode,omitempty"`

	// +kubebuilder:default=true
	K8sMode bool `json:"k8sMode,omitempty"`

	// +kubebuilder:validation:Enum=unprivileged;minimal;standard;privileged
	// +kubebuilder:default=standard
	PrivilegeMode PrivilegeMode `json:"privilegeMode,omitempty"`

	Security *SensorSecurityConfig `json:"security,omitempty"`

	Scanning *ScanningConfig `json:"scanning,omitempty"`

	Storage *StorageConfig `json:"storage,omitempty"`

	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// +kubebuilder:validation:Enum=AWS;AZURE;GCP;OCI;SELF_MANAGED_K8S
type CloudProvider string

const (
	CloudProviderAWS            CloudProvider = "AWS"
	CloudProviderAzure          CloudProvider = "AZURE"
	CloudProviderGCP            CloudProvider = "GCP"
	CloudProviderOCI            CloudProvider = "OCI"
	CloudProviderSelfManagedK8S CloudProvider = "SELF_MANAGED_K8S"
)

type ClusterSensorConfig struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Enum=AWS;AZURE;GCP;OCI;SELF_MANAGED_K8S
	// +kubebuilder:default=SELF_MANAGED_K8S
	CloudProvider CloudProvider `json:"cloudProvider,omitempty"`

	ClusterName string `json:"clusterName,omitempty"`

	ClusterID string `json:"clusterID,omitempty"`

	ClusterRegion string `json:"clusterRegion,omitempty"`

	// +kubebuilder:default=true
	K8sCompliance bool `json:"k8sCompliance,omitempty"`

	HostScanner *HostScannerConfig `json:"hostScanner,omitempty"`

	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	ExtraArgs []string `json:"extraArgs,omitempty"`
}

type HostScannerConfig struct {
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default=true
	RunOnMaster bool `json:"runOnMaster,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type AdmissionControllerConfig struct {
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	Image *ImageSpec `json:"image,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	Replicas *int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Enum=Fail;Ignore
	// +kubebuilder:default=Ignore
	FailurePolicy string `json:"failurePolicy,omitempty"`

	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	ExtraArgs []string `json:"extraArgs,omitempty"`
}

type RuntimeSensorConfig struct {
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	Image *ImageSpec `json:"image,omitempty"`

	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	ExtraArgs []string `json:"extraArgs,omitempty"`
}

type ContainerRuntimeConfig struct {
	// +kubebuilder:default=cri-o
	Type ContainerRuntimeType `json:"type,omitempty"`

	SocketPaths *RuntimeSocketPaths `json:"socketPaths,omitempty"`
}

type RuntimeSocketPaths struct {
	// +kubebuilder:default="/var/run/containerd/containerd.sock"
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_.-]+\.sock$`
	// +kubebuilder:validation:MaxLength=256
	Containerd string `json:"containerd,omitempty"`

	// +kubebuilder:default="/var/run/crio/crio.sock"
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_.-]+\.sock$`
	// +kubebuilder:validation:MaxLength=256
	CRIO string `json:"crio,omitempty"`

	// +kubebuilder:default="/var/run/docker.sock"
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_.-]+\.sock$`
	// +kubebuilder:validation:MaxLength=256
	Docker string `json:"docker,omitempty"`
}

type ScanningConfig struct {
	// +kubebuilder:validation:Enum=DynamicWithStaticScanningAsFallback;DynamicScanningOnly;StaticScanningOnly
	// +kubebuilder:default=DynamicWithStaticScanningAsFallback
	ScanningPolicy ScanningPolicy `json:"scanningPolicy,omitempty"`

	// +kubebuilder:default=true
	EnableImageScan bool `json:"enableImageScan,omitempty"`

	// +kubebuilder:default=true
	EnableContainerScan bool `json:"enableContainerScan,omitempty"`

	EnableMalwareDetection bool `json:"enableMalwareDetection,omitempty"`

	EnableSecretDetection bool `json:"enableSecretDetection,omitempty"`

	// +kubebuilder:default=true
	EnableScaScan bool `json:"enableScaScan,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=2
	ScanThreadPoolSize int `json:"scanThreadPoolSize,omitempty"`

	// +kubebuilder:default="10m"
	ContainerLaunchTimeout string `json:"containerLaunchTimeout,omitempty"`
}

type StorageConfig struct {
	// +kubebuilder:default=true
	UsePersistentStorage bool `json:"usePersistentStorage,omitempty"`

	StorageClass string `json:"storageClass,omitempty"`

	// +kubebuilder:default="10Gi"
	StorageSize string `json:"storageSize,omitempty"`
}

type SensorLoggingConfig struct {
	EnableConsoleLogs bool `json:"enableConsoleLogs,omitempty"`

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=3
	LogLevel int `json:"logLevel,omitempty"`

	// +kubebuilder:default="10M"
	LogFileSize string `json:"logFileSize,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	LogFilePurgeCount int `json:"logFilePurgeCount,omitempty"`
}

type SensorSecurityConfig struct {
	RuntimeSocketGroup *int64 `json:"runtimeSocketGroup,omitempty"`

	RunAsUser *int64 `json:"runAsUser,omitempty"`

	RunAsGroup *int64 `json:"runAsGroup,omitempty"`

	ReadOnlyRootFilesystem *bool `json:"readOnlyRootFilesystem,omitempty"`

	// +kubebuilder:validation:Enum=RuntimeDefault;Unconfined;Localhost
	SeccompProfile string `json:"seccompProfile,omitempty"`

	HostNetwork *bool `json:"hostNetwork,omitempty"`

	HostPID *bool `json:"hostPID,omitempty"`
}

type QualysContainerSecurityStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	DetectedRuntime string `json:"detectedRuntime,omitempty"`

	ContainerSensor *ComponentStatus `json:"containerSensor,omitempty"`

	ClusterSensor *ComponentStatus `json:"clusterSensor,omitempty"`

	AdmissionController *ComponentStatus `json:"admissionController,omitempty"`

	RuntimeSensor *ComponentStatus `json:"runtimeSensor,omitempty"`
}

type ComponentStatus struct {
	Enabled bool `json:"enabled"`

	Ready bool `json:"ready"`

	ResourceName string `json:"resourceName,omitempty"`

	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Container",type=string,JSONPath=`.status.containerSensor.ready`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.status.clusterSensor.ready`
// +kubebuilder:printcolumn:name="Admission",type=string,JSONPath=`.status.admissionController.ready`
// +kubebuilder:printcolumn:name="Runtime",type=string,JSONPath=`.status.runtimeSensor.ready`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type QualysContainerSecurity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QualysContainerSecuritySpec   `json:"spec,omitempty"`
	Status QualysContainerSecurityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type QualysContainerSecurityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QualysContainerSecurity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QualysContainerSecurity{}, &QualysContainerSecurityList{})
}

func (s *QualysContainerSecuritySpec) GetContainerSensor() ContainerSensorConfig {
	if s.ContainerSensor != nil {
		cfg := *s.ContainerSensor
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/qcs-sensor",
				Tag:        "latest",
				PullPolicy: corev1.PullIfNotPresent,
			}
		}
		if cfg.Mode == "" {
			cfg.Mode = ContainerSensorModeGeneral
		}
		if cfg.PrivilegeMode == "" {
			cfg.PrivilegeMode = PrivilegeModeStandard
		}
		return cfg
	}
	return ContainerSensorConfig{
		Enabled: true,
		Image: &ImageSpec{
			Repository: "qualys/qcs-sensor",
			Tag:        "latest",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Mode:          ContainerSensorModeGeneral,
		PrivilegeMode: PrivilegeModeStandard,
		K8sMode:       true,
		Scanning: &ScanningConfig{
			ScanningPolicy:         ScanningPolicyDynamicWithStaticFallback,
			EnableImageScan:        true,
			EnableContainerScan:    true,
			ScanThreadPoolSize:     2,
			ContainerLaunchTimeout: "10m",
		},
		Storage: &StorageConfig{
			UsePersistentStorage: true,
			StorageSize:          "10Gi",
		},
		Logging: &SensorLoggingConfig{
			LogLevel:          3,
			LogFileSize:       "10M",
			LogFilePurgeCount: 5,
		},
	}
}

func (c *ContainerSensorConfig) GetEffectiveHostNetwork() bool {
	if c.Security != nil && c.Security.HostNetwork != nil {
		return *c.Security.HostNetwork
	}
	return c.PrivilegeMode != PrivilegeModeUnprivileged
}

func (c *ContainerSensorConfig) GetEffectiveHostPID() bool {
	if c.Security != nil && c.Security.HostPID != nil {
		return *c.Security.HostPID
	}
	return c.PrivilegeMode != PrivilegeModeUnprivileged
}

func (c *ContainerSensorConfig) GetEffectiveReadOnlyRootFilesystem() bool {
	if c.Security != nil && c.Security.ReadOnlyRootFilesystem != nil {
		return *c.Security.ReadOnlyRootFilesystem
	}
	return c.PrivilegeMode == PrivilegeModeUnprivileged
}

func (c *ContainerSensorConfig) GetEffectiveRunAsUser() *int64 {
	if c.Security != nil && c.Security.RunAsUser != nil {
		return c.Security.RunAsUser
	}
	if c.PrivilegeMode == PrivilegeModeUnprivileged {
		uid := int64(65534)
		return &uid
	}
	uid := int64(0)
	return &uid
}

func (c *ContainerSensorConfig) GetEffectiveRunAsGroup() *int64 {
	if c.Security != nil && c.Security.RunAsGroup != nil {
		return c.Security.RunAsGroup
	}
	if c.PrivilegeMode == PrivilegeModeUnprivileged {
		gid := int64(65534)
		return &gid
	}
	return nil
}

func (c *ContainerSensorConfig) GetRuntimeSocketGroup() *int64 {
	if c.Security != nil && c.Security.RuntimeSocketGroup != nil {
		return c.Security.RuntimeSocketGroup
	}
	return nil
}

func (c *ContainerSensorConfig) GetEffectiveSeccompProfile() corev1.SeccompProfileType {
	if c.Security != nil && c.Security.SeccompProfile != "" {
		switch c.Security.SeccompProfile {
		case "RuntimeDefault":
			return corev1.SeccompProfileTypeRuntimeDefault
		case "Unconfined":
			return corev1.SeccompProfileTypeUnconfined
		case "Localhost":
			return corev1.SeccompProfileTypeLocalhost
		}
	}
	if c.PrivilegeMode == PrivilegeModeUnprivileged || c.PrivilegeMode == PrivilegeModeMinimal {
		return corev1.SeccompProfileTypeRuntimeDefault
	}
	return corev1.SeccompProfileTypeUnconfined
}

func (c *ContainerSensorConfig) IsContainerScanAllowed() bool {
	return c.PrivilegeMode != PrivilegeModeUnprivileged
}

func (c *ContainerSensorConfig) IsMalwareScanAllowed() bool {
	return c.PrivilegeMode == PrivilegeModeStandard
}

func (s *QualysContainerSecuritySpec) GetClusterSensor() ClusterSensorConfig {
	if s.ClusterSensor != nil {
		cfg := *s.ClusterSensor
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/cluster-sensor",
				Tag:        "1.4.0-0",
				PullPolicy: corev1.PullIfNotPresent,
			}
		}
		if cfg.Replicas == nil {
			replicas := int32(1)
			cfg.Replicas = &replicas
		}
		if cfg.CloudProvider == "" {
			cfg.CloudProvider = CloudProviderSelfManagedK8S
		}
		if cfg.Logging == nil {
			cfg.Logging = &SensorLoggingConfig{
				LogLevel: 3,
			}
		}
		return cfg
	}
	replicas := int32(1)
	return ClusterSensorConfig{
		Enabled: false,
		Image: &ImageSpec{
			Repository: "qualys/cluster-sensor",
			Tag:        "1.4.0-0",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Replicas:      &replicas,
		CloudProvider: CloudProviderSelfManagedK8S,
		K8sCompliance: true,
		HostScanner: &HostScannerConfig{
			Enabled:     true,
			RunOnMaster: true,
		},
		Logging: &SensorLoggingConfig{
			LogLevel: 3,
		},
	}
}

func (s *QualysContainerSecuritySpec) GetAdmissionController() AdmissionControllerConfig {
	if s.AdmissionController != nil {
		cfg := *s.AdmissionController
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/admission-controller",
				Tag:        "1.1.2-0",
				PullPolicy: corev1.PullIfNotPresent,
			}
		}
		if cfg.Replicas == nil {
			replicas := int32(2)
			cfg.Replicas = &replicas
		}
		if cfg.FailurePolicy == "" {
			cfg.FailurePolicy = "Ignore"
		}
		return cfg
	}
	replicas := int32(2)
	return AdmissionControllerConfig{
		Enabled: false,
		Image: &ImageSpec{
			Repository: "qualys/admission-controller",
			Tag:        "1.1.2-0",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Replicas:      &replicas,
		FailurePolicy: "Ignore",
		Logging: &SensorLoggingConfig{
			LogLevel: 3,
		},
	}
}

func (s *QualysContainerSecuritySpec) GetRuntimeSensor() RuntimeSensorConfig {
	if s.RuntimeSensor != nil {
		cfg := *s.RuntimeSensor
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/runtime-sensor",
				Tag:        "1.4.0-0",
				PullPolicy: corev1.PullIfNotPresent,
			}
		}
		return cfg
	}
	return RuntimeSensorConfig{
		Enabled: false,
		Image: &ImageSpec{
			Repository: "qualys/runtime-sensor",
			Tag:        "1.4.0-0",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Logging: &SensorLoggingConfig{
			LogLevel: 3,
		},
	}
}

func (s *QualysContainerSecuritySpec) GetContainerRuntime() ContainerRuntimeConfig {
	if s.ContainerRuntime != nil {
		return *s.ContainerRuntime
	}
	return ContainerRuntimeConfig{
		Type: ContainerRuntimeCRIO,
		SocketPaths: &RuntimeSocketPaths{
			CRIO: "/var/run/crio/crio.sock",
		},
	}
}

func (s *QualysContainerSecuritySpec) GetScheduling() SchedulingConfig {
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

func (s *QualysContainerSecuritySpec) IsAnyComponentEnabled() bool {
	if s.ContainerSensor != nil && s.ContainerSensor.Enabled {
		return true
	}
	if s.ClusterSensor != nil && s.ClusterSensor.Enabled {
		return true
	}
	if s.AdmissionController != nil && s.AdmissionController.Enabled {
		return true
	}
	if s.RuntimeSensor != nil && s.RuntimeSensor.Enabled {
		return true
	}
	return s.ContainerSensor == nil
}
