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
	ContainerRuntimeAuto       ContainerRuntimeType = "auto"
	ContainerRuntimeContainerd ContainerRuntimeType = "containerd"
	ContainerRuntimeCRIO       ContainerRuntimeType = "cri-o"
	ContainerRuntimeDocker     ContainerRuntimeType = "docker"
)

// ContainerSensorMode defines the operating mode for the container sensor
// +kubebuilder:validation:Enum=general;registry;cicd
type ContainerSensorMode string

const (
	ContainerSensorModeGeneral  ContainerSensorMode = "general"
	ContainerSensorModeRegistry ContainerSensorMode = "registry"
	ContainerSensorModeCICD     ContainerSensorMode = "cicd"
)

// QualysContainerSecuritySpec defines the desired state of QualysContainerSecurity.
type QualysContainerSecuritySpec struct {
	// PlatformConfigRef references the QualysPlatformConfig to use
	// +kubebuilder:validation:Required
	PlatformConfigRef PlatformConfigReference `json:"platformConfigRef"`

	// ContainerSensor configures the Container Security Sensor (DaemonSet)
	// Scans container images and running containers for vulnerabilities
	// +optional
	ContainerSensor *ContainerSensorConfig `json:"containerSensor,omitempty"`

	// ClusterSensor configures the Cluster Sensor (Deployment)
	// Monitors K8s API for cluster events, network activity, and workload inventory
	// +optional
	ClusterSensor *ClusterSensorConfig `json:"clusterSensor,omitempty"`

	// AdmissionController configures the Admission Controller (Deployment + Webhook)
	// Enforces security policies on resource creation/updates
	// +optional
	AdmissionController *AdmissionControllerConfig `json:"admissionController,omitempty"`

	// RuntimeSensor configures the Container Runtime Sensor (DaemonSet)
	// Uses eBPF to track file and process events in containers
	// +optional
	RuntimeSensor *RuntimeSensorConfig `json:"runtimeSensor,omitempty"`

	// ContainerRuntime specifies container runtime configuration (shared by sensors)
	// +optional
	ContainerRuntime *ContainerRuntimeConfig `json:"containerRuntime,omitempty"`

	// Scheduling defines pod scheduling options (shared by DaemonSet components)
	// +optional
	Scheduling *SchedulingConfig `json:"scheduling,omitempty"`

	// OpenShift defines OpenShift-specific settings
	// +optional
	OpenShift *OpenShiftConfig `json:"openshift,omitempty"`
}

// ContainerSensorConfig defines the Container Security Sensor configuration
type ContainerSensorConfig struct {
	// Enabled controls whether the Container Sensor is deployed
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Image defines the container image to use
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Mode specifies the operating mode: general, registry, or cicd
	// +kubebuilder:validation:Enum=general;registry;cicd
	// +kubebuilder:default=general
	Mode ContainerSensorMode `json:"mode,omitempty"`

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

	// Resources defines resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// UpdateStrategy defines the DaemonSet update strategy
	// +optional
	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	// ExtraArgs are additional arguments to pass to the sensor
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// ClusterSensorConfig defines the Cluster Sensor configuration
type ClusterSensorConfig struct {
	// Enabled controls whether the Cluster Sensor is deployed
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// Image defines the container image to use
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Replicas is the number of Cluster Sensor replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Logging contains logging configuration
	// +optional
	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	// Resources defines resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ExtraArgs are additional arguments to pass to the sensor
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// AdmissionControllerConfig defines the Admission Controller configuration
type AdmissionControllerConfig struct {
	// Enabled controls whether the Admission Controller is deployed
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Image defines the container image to use
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Replicas is the number of Admission Controller replicas
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=2
	Replicas *int32 `json:"replicas,omitempty"`

	// FailurePolicy defines webhook failure policy: Fail or Ignore
	// +kubebuilder:validation:Enum=Fail;Ignore
	// +kubebuilder:default=Ignore
	FailurePolicy string `json:"failurePolicy,omitempty"`

	// NamespaceSelector limits which namespaces are subject to admission control
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// Logging contains logging configuration
	// +optional
	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	// Resources defines resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ExtraArgs are additional arguments to pass to the controller
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// RuntimeSensorConfig defines the Container Runtime Sensor (eBPF) configuration
type RuntimeSensorConfig struct {
	// Enabled controls whether the Runtime Sensor is deployed
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// Image defines the container image to use
	// +optional
	Image *ImageSpec `json:"image,omitempty"`

	// Logging contains logging configuration
	// +optional
	Logging *SensorLoggingConfig `json:"logging,omitempty"`

	// Resources defines resource requirements
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// UpdateStrategy defines the DaemonSet update strategy
	// +optional
	UpdateStrategy *UpdateStrategyConfig `json:"updateStrategy,omitempty"`

	// ExtraArgs are additional arguments to pass to the sensor
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
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

// SensorLoggingConfig defines logging settings for sensors
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

	// ContainerSensor contains status for the Container Sensor
	// +optional
	ContainerSensor *ComponentStatus `json:"containerSensor,omitempty"`

	// ClusterSensor contains status for the Cluster Sensor
	// +optional
	ClusterSensor *ComponentStatus `json:"clusterSensor,omitempty"`

	// AdmissionController contains status for the Admission Controller
	// +optional
	AdmissionController *ComponentStatus `json:"admissionController,omitempty"`

	// RuntimeSensor contains status for the Runtime Sensor
	// +optional
	RuntimeSensor *ComponentStatus `json:"runtimeSensor,omitempty"`
}

// ComponentStatus represents the status of a deployed component
type ComponentStatus struct {
	// Enabled indicates if the component is enabled in spec
	Enabled bool `json:"enabled"`

	// Ready indicates if the component is ready
	Ready bool `json:"ready"`

	// ResourceName is the name of the managed resource (DaemonSet/Deployment)
	// +optional
	ResourceName string `json:"resourceName,omitempty"`

	// DesiredReplicas is the desired number of replicas/nodes
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// ReadyReplicas is the number of ready replicas/nodes
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Message provides additional status information
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Container",type=string,JSONPath=`.status.containerSensor.ready`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.status.clusterSensor.ready`
// +kubebuilder:printcolumn:name="Admission",type=string,JSONPath=`.status.admissionController.ready`
// +kubebuilder:printcolumn:name="Runtime",type=string,JSONPath=`.status.runtimeSensor.ready`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// QualysContainerSecurity is the Schema for the qualyscontainersecurities API.
// It manages deployment of Qualys Container Security components including
// Container Sensor, Cluster Sensor, Admission Controller, and Runtime Sensor.
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

// GetContainerSensor returns the container sensor config with defaults applied
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
		return cfg
	}
	return ContainerSensorConfig{
		Enabled: true,
		Image: &ImageSpec{
			Repository: "qualys/qcs-sensor",
			Tag:        "latest",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Mode:    ContainerSensorModeGeneral,
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

// GetClusterSensor returns the cluster sensor config with defaults applied
func (s *QualysContainerSecuritySpec) GetClusterSensor() ClusterSensorConfig {
	if s.ClusterSensor != nil {
		cfg := *s.ClusterSensor
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/cluster-sensor",
				Tag:        "latest",
				PullPolicy: corev1.PullIfNotPresent,
			}
		}
		if cfg.Replicas == nil {
			replicas := int32(1)
			cfg.Replicas = &replicas
		}
		return cfg
	}
	replicas := int32(1)
	return ClusterSensorConfig{
		Enabled: true,
		Image: &ImageSpec{
			Repository: "qualys/cluster-sensor",
			Tag:        "latest",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Replicas: &replicas,
		Logging: &SensorLoggingConfig{
			LogLevel: 3,
		},
	}
}

// GetAdmissionController returns the admission controller config with defaults applied
func (s *QualysContainerSecuritySpec) GetAdmissionController() AdmissionControllerConfig {
	if s.AdmissionController != nil {
		cfg := *s.AdmissionController
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/admission-controller",
				Tag:        "latest",
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
			Tag:        "latest",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Replicas:      &replicas,
		FailurePolicy: "Ignore",
		Logging: &SensorLoggingConfig{
			LogLevel: 3,
		},
	}
}

// GetRuntimeSensor returns the runtime sensor config with defaults applied
func (s *QualysContainerSecuritySpec) GetRuntimeSensor() RuntimeSensorConfig {
	if s.RuntimeSensor != nil {
		cfg := *s.RuntimeSensor
		if cfg.Image == nil {
			cfg.Image = &ImageSpec{
				Repository: "qualys/runtime-sensor",
				Tag:        "latest",
				PullPolicy: corev1.PullIfNotPresent,
			}
		}
		return cfg
	}
	return RuntimeSensorConfig{
		Enabled: false,
		Image: &ImageSpec{
			Repository: "qualys/runtime-sensor",
			Tag:        "latest",
			PullPolicy: corev1.PullIfNotPresent,
		},
		Logging: &SensorLoggingConfig{
			LogLevel: 3,
		},
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

// IsAnyComponentEnabled returns true if any component is enabled
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
