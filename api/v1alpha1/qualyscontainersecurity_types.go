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

// ContainerRuntimeType defines the container runtime type
// +kubebuilder:validation:Enum=auto;containerd;cri-o;docker
type ContainerRuntimeType string

const (
	// ContainerRuntimeAuto enables automatic runtime detection
	ContainerRuntimeAuto ContainerRuntimeType = "auto"
	// ContainerRuntimeContainerd specifies containerd runtime
	ContainerRuntimeContainerd ContainerRuntimeType = "containerd"
	// ContainerRuntimeCRIO specifies CRI-O runtime
	ContainerRuntimeCRIO ContainerRuntimeType = "cri-o"
	// ContainerRuntimeDocker specifies Docker runtime
	ContainerRuntimeDocker ContainerRuntimeType = "docker"
)

// QualysContainerSecuritySpec defines the desired state of QualysContainerSecurity.
type QualysContainerSecuritySpec struct {
	// PlatformConfigRef references the QualysPlatformConfig to use
	// +kubebuilder:validation:Required
	PlatformConfigRef PlatformConfigReference `json:"platformConfigRef"`

	// Image defines the container image to use for the sensor
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// ContainerRuntime specifies container runtime configuration
	// +optional
	ContainerRuntime *ContainerRuntimeConfig `json:"containerRuntime,omitempty"`

	// SensorConfig contains sensor-specific configuration
	// +optional
	SensorConfig *SensorConfig `json:"sensorConfig,omitempty"`

	// Scheduling defines pod scheduling options
	// +optional
	Scheduling *SchedulingConfig `json:"scheduling,omitempty"`

	// Resources defines resource requirements for the sensor container
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// UpdateStrategy defines the DaemonSet update strategy
	// +optional
	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	// OpenShift defines OpenShift-specific settings
	// +optional
	OpenShift *OpenShiftConfig `json:"openshift,omitempty"`
}

// ContainerRuntimeConfig defines container runtime settings
type ContainerRuntimeConfig struct {
	// Type specifies the container runtime type
	// +kubebuilder:default=auto
	Type ContainerRuntimeType `json:"type,omitempty"`

	// SocketPaths overrides default socket paths for each runtime
	// +optional
	SocketPaths *RuntimeSocketPaths `json:"socketPaths,omitempty"`
}

// RuntimeSocketPaths defines custom socket paths for container runtimes
type RuntimeSocketPaths struct {
	// Containerd socket path
	// +kubebuilder:default="/var/run/containerd/containerd.sock"
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_.-]+\.sock$`
	// +kubebuilder:validation:MaxLength=256
	Containerd string `json:"containerd,omitempty"`

	// CRIO socket path
	// +kubebuilder:default="/var/run/crio/crio.sock"
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_.-]+\.sock$`
	// +kubebuilder:validation:MaxLength=256
	CRIO string `json:"crio,omitempty"`

	// Docker socket path
	// +kubebuilder:default="/var/run/docker.sock"
	// +kubebuilder:validation:Pattern=`^/[a-zA-Z0-9/_.-]+\.sock$`
	// +kubebuilder:validation:MaxLength=256
	Docker string `json:"docker,omitempty"`
}

// SensorConfig defines Qualys Container Security Sensor configuration
type SensorConfig struct {
	// Mode specifies the operating mode
	// +kubebuilder:validation:Enum=general;registry;cicd
	// +kubebuilder:default=general
	Mode string `json:"mode,omitempty"`

	// K8sMode enables Kubernetes integration
	// +kubebuilder:default=true
	K8sMode bool `json:"k8sMode,omitempty"`

	// Scanning contains scanning configuration
	// +optional
	Scanning *ScanningConfig `json:"scanning,omitempty"`

	// Storage contains persistent storage configuration
	// +optional
	Storage *StorageConfig `json:"storage,omitempty"`

	// Logging contains logging configuration
	// +optional
	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	// ExtraArgs are additional arguments to pass to the sensor
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// ScanningConfig defines scanning options
type ScanningConfig struct {
	// EnableImageScan enables container image scanning
	// +kubebuilder:default=true
	EnableImageScan bool `json:"enableImageScan,omitempty"`

	// EnableContainerScan enables running container scanning
	// +kubebuilder:default=true
	EnableContainerScan bool `json:"enableContainerScan,omitempty"`

	// EnableMalwareDetection enables malware detection
	// +optional
	EnableMalwareDetection bool `json:"enableMalwareDetection,omitempty"`

	// EnableSecretDetection enables secret detection in images
	// +optional
	EnableSecretDetection bool `json:"enableSecretDetection,omitempty"`

	// ScanThreadPoolSize is the number of concurrent scan threads
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=2
	ScanThreadPoolSize int `json:"scanThreadPoolSize,omitempty"`

	// ContainerLaunchTimeout is the timeout for container launch detection
	// +kubebuilder:default="10m"
	ContainerLaunchTimeout string `json:"containerLaunchTimeout,omitempty"`
}

// StorageConfig defines persistent storage settings
type StorageConfig struct {
	// UsePersistentStorage enables persistent storage for scan data
	// +kubebuilder:default=true
	UsePersistentStorage bool `json:"usePersistentStorage,omitempty"`

	// StorageClass is the storage class to use (empty uses default)
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// StorageSize is the size of the persistent volume
	// +kubebuilder:default="10Gi"
	StorageSize string `json:"storageSize,omitempty"`
}

// SensorLoggingConfig defines logging settings for the sensor
type SensorLoggingConfig struct {
	// EnableConsoleLogs enables logging to console
	// +optional
	EnableConsoleLogs bool `json:"enableConsoleLogs,omitempty"`

	// LogLevel sets the log verbosity (0-5)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:default=3
	LogLevel int `json:"logLevel,omitempty"`

	// LogFileSize is the maximum log file size
	// +kubebuilder:default="10M"
	LogFileSize string `json:"logFileSize,omitempty"`

	// LogFilePurgeCount is the number of old log files to keep
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	LogFilePurgeCount int `json:"logFilePurgeCount,omitempty"`
}

// QualysContainerSecurityStatus defines the observed state of QualysContainerSecurity.
type QualysContainerSecurityStatus struct {
	// Conditions represent the latest available observations of the resource's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DetectedRuntime is the detected container runtime
	// +optional
	DetectedRuntime string `json:"detectedRuntime,omitempty"`

	// DesiredNumberScheduled is the total number of nodes that should be running the sensor
	// +optional
	DesiredNumberScheduled int32 `json:"desiredNumberScheduled,omitempty"`

	// CurrentNumberScheduled is the number of nodes running at least one sensor pod
	// +optional
	CurrentNumberScheduled int32 `json:"currentNumberScheduled,omitempty"`

	// NumberReady is the number of nodes with ready sensor pods
	// +optional
	NumberReady int32 `json:"numberReady,omitempty"`

	// NumberAvailable is the number of nodes with available sensor pods
	// +optional
	NumberAvailable int32 `json:"numberAvailable,omitempty"`

	// UpdatedNumberScheduled is the number of nodes running updated sensor pods
	// +optional
	UpdatedNumberScheduled int32 `json:"updatedNumberScheduled,omitempty"`

	// NumberMisscheduled is the number of nodes running sensor pods that shouldn't be
	// +optional
	NumberMisscheduled int32 `json:"numberMisscheduled,omitempty"`

	// DaemonSetName is the name of the managed DaemonSet
	// +optional
	DaemonSetName string `json:"daemonSetName,omitempty"`

	// ScanningStats contains aggregated scanning statistics
	// +optional
	ScanningStats *ScanningStats `json:"scanningStats,omitempty"`
}

// ScanningStats contains aggregated scanning statistics
type ScanningStats struct {
	// ImagesScanned is the total number of images scanned
	// +optional
	ImagesScanned int64 `json:"imagesScanned,omitempty"`

	// ContainersScanned is the total number of containers scanned
	// +optional
	ContainersScanned int64 `json:"containersScanned,omitempty"`

	// LastScanTime is the timestamp of the last scan
	// +optional
	LastScanTime *metav1.Time `json:"lastScanTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Runtime",type=string,JSONPath=`.status.detectedRuntime`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredNumberScheduled`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.numberReady`
// +kubebuilder:printcolumn:name="Available",type=integer,JSONPath=`.status.numberAvailable`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type=="Available")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QualysContainerSecurity is the Schema for the qualyscontainersecurities API.
// It manages the deployment of Qualys Container Security Sensor as a DaemonSet.
type QualysContainerSecurity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QualysContainerSecuritySpec   `json:"spec,omitempty"`
	Status QualysContainerSecurityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QualysContainerSecurityList contains a list of QualysContainerSecurity.
type QualysContainerSecurityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QualysContainerSecurity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QualysContainerSecurity{}, &QualysContainerSecurityList{})
}

func (s *QualysContainerSecuritySpec) GetImage() ImageSpec {
	if s.Image != nil {
		img := *s.Image
		if img.Repository == "" {
			img.Repository = "qualys/qcs-sensor"
		}
		if img.Tag == "" {
			img.Tag = "latest"
		}
		if img.PullPolicy == "" {
			img.PullPolicy = corev1.PullIfNotPresent
		}
		return img
	}
	return ImageSpec{
		Repository: "qualys/qcs-sensor",
		Tag:        "latest",
		PullPolicy: corev1.PullIfNotPresent,
	}
}

// GetContainerRuntime returns the container runtime config with defaults applied
func (s *QualysContainerSecuritySpec) GetContainerRuntime() ContainerRuntimeConfig {
	if s.ContainerRuntime != nil {
		return *s.ContainerRuntime
	}
	return ContainerRuntimeConfig{
		Type: ContainerRuntimeAuto,
		SocketPaths: &RuntimeSocketPaths{
			Containerd: "/var/run/containerd/containerd.sock",
			CRIO:       "/var/run/crio/crio.sock",
			Docker:     "/var/run/docker.sock",
		},
	}
}

// GetScheduling returns the scheduling config with defaults applied
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

// GetSensorConfig returns the sensor config with defaults applied
func (s *QualysContainerSecuritySpec) GetSensorConfig() SensorConfig {
	if s.SensorConfig != nil {
		return *s.SensorConfig
	}
	return SensorConfig{
		Mode:    "general",
		K8sMode: true,
		Scanning: &ScanningConfig{
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

// GetUpdateStrategy returns the update strategy with defaults applied
func (s *QualysContainerSecuritySpec) GetUpdateStrategy() UpdateStrategyConfig {
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
