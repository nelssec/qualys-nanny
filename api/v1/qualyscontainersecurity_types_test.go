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
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetContainerSensorWithNil(t *testing.T) {
	spec := &QualysContainerSecuritySpec{}
	cfg := spec.GetContainerSensor()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if cfg.Mode != ContainerSensorModeGeneral {
		t.Errorf("expected Mode 'general', got '%s'", cfg.Mode)
	}
	if cfg.PrivilegeMode != PrivilegeModeStandard {
		t.Errorf("expected PrivilegeMode 'standard', got '%s'", cfg.PrivilegeMode)
	}
	if !cfg.K8sMode {
		t.Error("expected K8sMode to be true by default")
	}
	if cfg.Image == nil {
		t.Error("expected Image to not be nil")
	}
	if cfg.Scanning == nil {
		t.Error("expected Scanning to not be nil")
	}
	if cfg.Storage == nil {
		t.Error("expected Storage to not be nil")
	}
	if cfg.Logging == nil {
		t.Error("expected Logging to not be nil")
	}
}

func TestGetContainerSensorWithPartialConfig(t *testing.T) {
	spec := &QualysContainerSecuritySpec{
		ContainerSensor: &ContainerSensorConfig{
			Enabled: true,
			K8sMode: true,
		},
	}
	cfg := spec.GetContainerSensor()

	if cfg.Image == nil {
		t.Error("expected Image to be filled in")
	}
	if cfg.Mode != ContainerSensorModeGeneral {
		t.Errorf("expected Mode to default to 'general', got '%s'", cfg.Mode)
	}
	if cfg.PrivilegeMode != PrivilegeModeStandard {
		t.Errorf("expected PrivilegeMode to default to 'standard', got '%s'", cfg.PrivilegeMode)
	}
}

func TestGetContainerSensorWithFullConfig(t *testing.T) {
	spec := &QualysContainerSecuritySpec{
		ContainerSensor: &ContainerSensorConfig{
			Enabled: true,
			Image: &ImageSpec{
				Repository: "custom/sensor",
				Tag:        "v1.0.0",
				PullPolicy: corev1.PullAlways,
			},
			Mode:          ContainerSensorModeRegistry,
			PrivilegeMode: PrivilegeModeMinimal,
		},
	}
	cfg := spec.GetContainerSensor()

	if cfg.Image.Repository != "custom/sensor" {
		t.Errorf("expected custom repository, got '%s'", cfg.Image.Repository)
	}
	if cfg.Mode != ContainerSensorModeRegistry {
		t.Errorf("expected Mode 'registry', got '%s'", cfg.Mode)
	}
	if cfg.PrivilegeMode != PrivilegeModeMinimal {
		t.Errorf("expected PrivilegeMode 'minimal', got '%s'", cfg.PrivilegeMode)
	}
}

func TestGetEffectiveHostNetwork(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue bool
	}{
		{
			name: "unprivileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
			},
			expectedValue: false,
		},
		{
			name: "minimal mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeMinimal,
			},
			expectedValue: true,
		},
		{
			name: "standard mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
			},
			expectedValue: true,
		},
		{
			name: "privileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModePrivileged,
			},
			expectedValue: true,
		},
		{
			name: "explicit override to false",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
				Security: &SensorSecurityConfig{
					HostNetwork: boolPtr(false),
				},
			},
			expectedValue: false,
		},
		{
			name: "explicit override to true",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
				Security: &SensorSecurityConfig{
					HostNetwork: boolPtr(true),
				},
			},
			expectedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetEffectiveHostNetwork()
			if result != tt.expectedValue {
				t.Errorf("expected %v, got %v", tt.expectedValue, result)
			}
		})
	}
}

func TestGetEffectiveHostPID(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue bool
	}{
		{
			name: "unprivileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
			},
			expectedValue: false,
		},
		{
			name: "standard mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
			},
			expectedValue: true,
		},
		{
			name: "explicit override",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
				Security: &SensorSecurityConfig{
					HostPID: boolPtr(true),
				},
			},
			expectedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetEffectiveHostPID()
			if result != tt.expectedValue {
				t.Errorf("expected %v, got %v", tt.expectedValue, result)
			}
		})
	}
}

func TestGetEffectiveReadOnlyRootFilesystem(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue bool
	}{
		{
			name: "unprivileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
			},
			expectedValue: true,
		},
		{
			name: "standard mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
			},
			expectedValue: false,
		},
		{
			name: "explicit override",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
				Security: &SensorSecurityConfig{
					ReadOnlyRootFilesystem: boolPtr(true),
				},
			},
			expectedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetEffectiveReadOnlyRootFilesystem()
			if result != tt.expectedValue {
				t.Errorf("expected %v, got %v", tt.expectedValue, result)
			}
		})
	}
}

func TestGetEffectiveRunAsUser(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue int64
	}{
		{
			name: "unprivileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
			},
			expectedValue: 65534,
		},
		{
			name: "standard mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
			},
			expectedValue: 0,
		},
		{
			name: "explicit override",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
				Security: &SensorSecurityConfig{
					RunAsUser: int64Ptr(1000),
				},
			},
			expectedValue: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetEffectiveRunAsUser()
			if result == nil {
				t.Error("expected result to not be nil")
				return
			}
			if *result != tt.expectedValue {
				t.Errorf("expected %d, got %d", tt.expectedValue, *result)
			}
		})
	}
}

func TestGetEffectiveRunAsGroup(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue *int64
	}{
		{
			name: "unprivileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
			},
			expectedValue: int64Ptr(65534),
		},
		{
			name: "standard mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
			},
			expectedValue: nil,
		},
		{
			name: "explicit override",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
				Security: &SensorSecurityConfig{
					RunAsGroup: int64Ptr(1000),
				},
			},
			expectedValue: int64Ptr(1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetEffectiveRunAsGroup()
			if tt.expectedValue == nil {
				if result != nil {
					t.Errorf("expected nil, got %d", *result)
				}
			} else {
				if result == nil {
					t.Error("expected non-nil result")
					return
				}
				if *result != *tt.expectedValue {
					t.Errorf("expected %d, got %d", *tt.expectedValue, *result)
				}
			}
		})
	}
}

func TestGetRuntimeSocketGroup(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue *int64
	}{
		{
			name:          "no security config",
			config:        &ContainerSensorConfig{},
			expectedValue: nil,
		},
		{
			name: "no runtime socket group",
			config: &ContainerSensorConfig{
				Security: &SensorSecurityConfig{},
			},
			expectedValue: nil,
		},
		{
			name: "with runtime socket group",
			config: &ContainerSensorConfig{
				Security: &SensorSecurityConfig{
					RuntimeSocketGroup: int64Ptr(993),
				},
			},
			expectedValue: int64Ptr(993),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetRuntimeSocketGroup()
			if tt.expectedValue == nil {
				if result != nil {
					t.Errorf("expected nil, got %d", *result)
				}
			} else {
				if result == nil {
					t.Error("expected non-nil result")
					return
				}
				if *result != *tt.expectedValue {
					t.Errorf("expected %d, got %d", *tt.expectedValue, *result)
				}
			}
		})
	}
}

func TestGetEffectiveSeccompProfile(t *testing.T) {
	tests := []struct {
		name          string
		config        *ContainerSensorConfig
		expectedValue corev1.SeccompProfileType
	}{
		{
			name: "unprivileged mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
			},
			expectedValue: corev1.SeccompProfileTypeRuntimeDefault,
		},
		{
			name: "minimal mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeMinimal,
			},
			expectedValue: corev1.SeccompProfileTypeRuntimeDefault,
		},
		{
			name: "standard mode",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
			},
			expectedValue: corev1.SeccompProfileTypeUnconfined,
		},
		{
			name: "explicit RuntimeDefault",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
				Security: &SensorSecurityConfig{
					SeccompProfile: "RuntimeDefault",
				},
			},
			expectedValue: corev1.SeccompProfileTypeRuntimeDefault,
		},
		{
			name: "explicit Unconfined",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeUnprivileged,
				Security: &SensorSecurityConfig{
					SeccompProfile: "Unconfined",
				},
			},
			expectedValue: corev1.SeccompProfileTypeUnconfined,
		},
		{
			name: "explicit Localhost",
			config: &ContainerSensorConfig{
				PrivilegeMode: PrivilegeModeStandard,
				Security: &SensorSecurityConfig{
					SeccompProfile: "Localhost",
				},
			},
			expectedValue: corev1.SeccompProfileTypeLocalhost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetEffectiveSeccompProfile()
			if result != tt.expectedValue {
				t.Errorf("expected %s, got %s", tt.expectedValue, result)
			}
		})
	}
}

func TestIsContainerScanAllowed(t *testing.T) {
	tests := []struct {
		name          string
		privilegeMode PrivilegeMode
		expected      bool
	}{
		{
			name:          "unprivileged",
			privilegeMode: PrivilegeModeUnprivileged,
			expected:      false,
		},
		{
			name:          "minimal",
			privilegeMode: PrivilegeModeMinimal,
			expected:      true,
		},
		{
			name:          "standard",
			privilegeMode: PrivilegeModeStandard,
			expected:      true,
		},
		{
			name:          "privileged",
			privilegeMode: PrivilegeModePrivileged,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ContainerSensorConfig{PrivilegeMode: tt.privilegeMode}
			result := cfg.IsContainerScanAllowed()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsMalwareScanAllowed(t *testing.T) {
	tests := []struct {
		name          string
		privilegeMode PrivilegeMode
		expected      bool
	}{
		{
			name:          "unprivileged",
			privilegeMode: PrivilegeModeUnprivileged,
			expected:      false,
		},
		{
			name:          "minimal",
			privilegeMode: PrivilegeModeMinimal,
			expected:      false,
		},
		{
			name:          "standard",
			privilegeMode: PrivilegeModeStandard,
			expected:      true,
		},
		{
			name:          "privileged",
			privilegeMode: PrivilegeModePrivileged,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ContainerSensorConfig{PrivilegeMode: tt.privilegeMode}
			result := cfg.IsMalwareScanAllowed()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetClusterSensor(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{}
		cfg := spec.GetClusterSensor()

		if cfg.Enabled {
			t.Error("expected Enabled to be false by default")
		}
		if cfg.Image == nil {
			t.Error("expected Image to not be nil")
		}
		if cfg.Replicas == nil || *cfg.Replicas != 1 {
			t.Error("expected Replicas to be 1")
		}
		if cfg.CloudProvider != CloudProviderSelfManagedK8S {
			t.Errorf("expected CloudProvider 'SELF_MANAGED_K8S', got '%s'", cfg.CloudProvider)
		}
	})

	t.Run("partial config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{
			ClusterSensor: &ClusterSensorConfig{
				Enabled: true,
			},
		}
		cfg := spec.GetClusterSensor()

		if !cfg.Enabled {
			t.Error("expected Enabled to be true")
		}
		if cfg.Image == nil {
			t.Error("expected Image to be filled in")
		}
		if cfg.Replicas == nil || *cfg.Replicas != 1 {
			t.Error("expected Replicas to default to 1")
		}
	})
}

func TestGetAdmissionController(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{}
		cfg := spec.GetAdmissionController()

		if cfg.Enabled {
			t.Error("expected Enabled to be false by default")
		}
		if cfg.Image == nil {
			t.Error("expected Image to not be nil")
		}
		if cfg.Replicas == nil || *cfg.Replicas != 2 {
			t.Error("expected Replicas to be 2")
		}
		if cfg.FailurePolicy != "Ignore" {
			t.Errorf("expected FailurePolicy 'Ignore', got '%s'", cfg.FailurePolicy)
		}
	})

	t.Run("partial config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{
			AdmissionController: &AdmissionControllerConfig{
				Enabled: true,
			},
		}
		cfg := spec.GetAdmissionController()

		if !cfg.Enabled {
			t.Error("expected Enabled to be true")
		}
		if cfg.FailurePolicy != "Ignore" {
			t.Errorf("expected FailurePolicy to default to 'Ignore', got '%s'", cfg.FailurePolicy)
		}
	})
}

func TestGetRuntimeSensor(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{}
		cfg := spec.GetRuntimeSensor()

		if cfg.Enabled {
			t.Error("expected Enabled to be false by default")
		}
		if cfg.Image == nil {
			t.Error("expected Image to not be nil")
		}
	})

	t.Run("partial config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{
			RuntimeSensor: &RuntimeSensorConfig{
				Enabled: true,
			},
		}
		cfg := spec.GetRuntimeSensor()

		if !cfg.Enabled {
			t.Error("expected Enabled to be true")
		}
		if cfg.Image == nil {
			t.Error("expected Image to be filled in")
		}
	})
}

func TestGetContainerRuntime(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{}
		cfg := spec.GetContainerRuntime()

		if cfg.Type != ContainerRuntimeCRIO {
			t.Errorf("expected Type 'cri-o', got '%s'", cfg.Type)
		}
		if cfg.SocketPaths == nil || cfg.SocketPaths.CRIO != "/var/run/crio/crio.sock" {
			t.Error("expected CRIO socket path to be set")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{
			ContainerRuntime: &ContainerRuntimeConfig{
				Type: ContainerRuntimeContainerd,
			},
		}
		cfg := spec.GetContainerRuntime()

		if cfg.Type != ContainerRuntimeContainerd {
			t.Errorf("expected Type 'containerd', got '%s'", cfg.Type)
		}
	})
}

func TestGetScheduling(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{}
		cfg := spec.GetScheduling()

		if cfg.NodeSelector == nil || cfg.NodeSelector["kubernetes.io/os"] != "linux" {
			t.Error("expected default node selector")
		}
		if len(cfg.Tolerations) != 2 {
			t.Error("expected 2 default tolerations")
		}
		if cfg.PriorityClassName != "system-node-critical" {
			t.Errorf("expected PriorityClassName 'system-node-critical', got '%s'", cfg.PriorityClassName)
		}
	})

	t.Run("partial config", func(t *testing.T) {
		spec := &QualysContainerSecuritySpec{
			Scheduling: &SchedulingConfig{},
		}
		cfg := spec.GetScheduling()

		if cfg.NodeSelector == nil {
			t.Error("expected node selector to be filled in")
		}
		if len(cfg.Tolerations) == 0 {
			t.Error("expected tolerations to be filled in")
		}
		if cfg.PriorityClassName != "system-node-critical" {
			t.Errorf("expected PriorityClassName to default to 'system-node-critical', got '%s'", cfg.PriorityClassName)
		}
	})
}

func TestIsAnyComponentEnabled(t *testing.T) {
	tests := []struct {
		name     string
		spec     *QualysContainerSecuritySpec
		expected bool
	}{
		{
			name:     "nil container sensor defaults to enabled",
			spec:     &QualysContainerSecuritySpec{},
			expected: true,
		},
		{
			name: "container sensor enabled",
			spec: &QualysContainerSecuritySpec{
				ContainerSensor: &ContainerSensorConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "cluster sensor enabled",
			spec: &QualysContainerSecuritySpec{
				ContainerSensor: &ContainerSensorConfig{Enabled: false},
				ClusterSensor:   &ClusterSensorConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "admission controller enabled",
			spec: &QualysContainerSecuritySpec{
				ContainerSensor:     &ContainerSensorConfig{Enabled: false},
				AdmissionController: &AdmissionControllerConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "runtime sensor enabled",
			spec: &QualysContainerSecuritySpec{
				ContainerSensor: &ContainerSensorConfig{Enabled: false},
				RuntimeSensor:   &RuntimeSensorConfig{Enabled: true},
			},
			expected: true,
		},
		{
			name: "all disabled",
			spec: &QualysContainerSecuritySpec{
				ContainerSensor:     &ContainerSensorConfig{Enabled: false},
				ClusterSensor:       &ClusterSensorConfig{Enabled: false},
				AdmissionController: &AdmissionControllerConfig{Enabled: false},
				RuntimeSensor:       &RuntimeSensorConfig{Enabled: false},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.IsAnyComponentEnabled()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
