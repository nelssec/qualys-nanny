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

package platform

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectContainerRuntime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ContainerRuntime
	}{
		{
			name:     "containerd runtime",
			input:    "containerd://1.6.0",
			expected: RuntimeContainerd,
		},
		{
			name:     "cri-o runtime",
			input:    "cri-o://1.23.0",
			expected: RuntimeCRIO,
		},
		{
			name:     "docker runtime",
			input:    "docker://20.10.0",
			expected: RuntimeDocker,
		},
		{
			name:     "unknown runtime",
			input:    "podman://4.0.0",
			expected: RuntimeUnknown,
		},
		{
			name:     "empty string",
			input:    "",
			expected: RuntimeUnknown,
		},
		{
			name:     "no colon",
			input:    "containerd",
			expected: RuntimeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectContainerRuntime(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDetectNodeOS(t *testing.T) {
	tests := []struct {
		name           string
		osImage        string
		runtimeVersion string
		expectedOS     OSType
		expectedRT     ContainerRuntime
	}{
		{
			name:           "CoreOS",
			osImage:        "Red Hat Enterprise Linux CoreOS 413.92.202310170157-0",
			runtimeVersion: "cri-o://1.27.1",
			expectedOS:     OSTypeCoreOS,
			expectedRT:     RuntimeCRIO,
		},
		{
			name:           "RHCOS",
			osImage:        "RHCOS 4.12",
			runtimeVersion: "cri-o://1.25.0",
			expectedOS:     OSTypeCoreOS,
			expectedRT:     RuntimeCRIO,
		},
		{
			name:           "Fedora CoreOS",
			osImage:        "Fedora CoreOS 38.20230806",
			runtimeVersion: "containerd://1.7.0",
			expectedOS:     OSTypeCoreOS,
			expectedRT:     RuntimeContainerd,
		},
		{
			name:           "Flatcar",
			osImage:        "Flatcar Container Linux by Kinvolk",
			runtimeVersion: "docker://20.10.0",
			expectedOS:     OSTypeCoreOS,
			expectedRT:     RuntimeDocker,
		},
		{
			name:           "RHEL",
			osImage:        "Red Hat Enterprise Linux 8.5",
			runtimeVersion: "cri-o://1.23.0",
			expectedOS:     OSTypeRHEL,
			expectedRT:     RuntimeCRIO,
		},
		{
			name:           "CentOS",
			osImage:        "CentOS Linux 7",
			runtimeVersion: "docker://19.03.0",
			expectedOS:     OSTypeRHEL,
			expectedRT:     RuntimeDocker,
		},
		{
			name:           "Rocky Linux",
			osImage:        "Rocky Linux 8.6",
			runtimeVersion: "containerd://1.6.0",
			expectedOS:     OSTypeRHEL,
			expectedRT:     RuntimeContainerd,
		},
		{
			name:           "AlmaLinux",
			osImage:        "AlmaLinux 8.7",
			runtimeVersion: "cri-o://1.25.0",
			expectedOS:     OSTypeRHEL,
			expectedRT:     RuntimeCRIO,
		},
		{
			name:           "Oracle Linux",
			osImage:        "Oracle Linux Server 8.6",
			runtimeVersion: "cri-o://1.25.0",
			expectedOS:     OSTypeRHEL,
			expectedRT:     RuntimeCRIO,
		},
		{
			name:           "Amazon Linux",
			osImage:        "Amazon Linux 2",
			runtimeVersion: "containerd://1.6.6",
			expectedOS:     OSTypeRHEL,
			expectedRT:     RuntimeContainerd,
		},
		{
			name:           "Ubuntu",
			osImage:        "Ubuntu 22.04.2 LTS",
			runtimeVersion: "containerd://1.6.15",
			expectedOS:     OSTypeDebian,
			expectedRT:     RuntimeContainerd,
		},
		{
			name:           "Debian",
			osImage:        "Debian GNU/Linux 11 (bullseye)",
			runtimeVersion: "containerd://1.5.0",
			expectedOS:     OSTypeDebian,
			expectedRT:     RuntimeContainerd,
		},
		{
			name:           "Unknown OS",
			osImage:        "Windows Server 2022",
			runtimeVersion: "containerd://1.6.0",
			expectedOS:     OSTypeUnknown,
			expectedRT:     RuntimeContainerd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						OSImage:                 tt.osImage,
						ContainerRuntimeVersion: tt.runtimeVersion,
						KernelVersion:           "5.15.0",
						Architecture:            "amd64",
					},
				},
			}

			info := DetectNodeOS(node)

			if info.OSType != tt.expectedOS {
				t.Errorf("expected OSType %s, got %s", tt.expectedOS, info.OSType)
			}
			if info.ContainerRuntime != tt.expectedRT {
				t.Errorf("expected ContainerRuntime %s, got %s", tt.expectedRT, info.ContainerRuntime)
			}
			if info.OSImage != tt.osImage {
				t.Errorf("expected OSImage '%s', got '%s'", tt.osImage, info.OSImage)
			}
			if info.KernelVersion != "5.15.0" {
				t.Errorf("expected KernelVersion '5.15.0', got '%s'", info.KernelVersion)
			}
			if info.Architecture != "amd64" {
				t.Errorf("expected Architecture 'amd64', got '%s'", info.Architecture)
			}
		})
	}
}

func TestIsCoreOSNode(t *testing.T) {
	tests := []struct {
		name     string
		osImage  string
		expected bool
	}{
		{
			name:     "CoreOS node",
			osImage:  "Red Hat Enterprise Linux CoreOS 413.92",
			expected: true,
		},
		{
			name:     "RHEL node",
			osImage:  "Red Hat Enterprise Linux 8.5",
			expected: false,
		},
		{
			name:     "Ubuntu node",
			osImage:  "Ubuntu 22.04.2 LTS",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						OSImage:                 tt.osImage,
						ContainerRuntimeVersion: "cri-o://1.27.0",
					},
				},
			}

			result := IsCoreOSNode(node)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestResetCache(t *testing.T) {
	ResetCache()
	detectionMutex.RLock()
	if isOpenShift != nil {
		t.Error("expected isOpenShift to be nil after ResetCache")
	}
	detectionMutex.RUnlock()
}

func TestSCCGVR(t *testing.T) {
	if SCCGVR.Group != "security.openshift.io" {
		t.Errorf("expected Group 'security.openshift.io', got '%s'", SCCGVR.Group)
	}
	if SCCGVR.Version != "v1" {
		t.Errorf("expected Version 'v1', got '%s'", SCCGVR.Version)
	}
	if SCCGVR.Resource != "securitycontextconstraints" {
		t.Errorf("expected Resource 'securitycontextconstraints', got '%s'", SCCGVR.Resource)
	}
}
