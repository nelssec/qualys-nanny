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

package resources

import (
	"testing"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
)

func TestBuildContainerSensorConfigMap(t *testing.T) {
	platformConfig := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Platform: qualysv1.PlatformSettings{
				ServerUri: "https://test.qualys.com/ContainerSensor",
			},
		},
	}

	sensorConfig := qualysv1.ContainerSensorConfig{
		Mode:    qualysv1.ContainerSensorModeGeneral,
		K8sMode: true,
		Scanning: &qualysv1.ScanningConfig{
			EnableImageScan:     true,
			EnableContainerScan: true,
			ScanThreadPoolSize:  4,
		},
		Logging: &qualysv1.SensorLoggingConfig{
			LogLevel:          3,
			EnableConsoleLogs: true,
			LogFileSize:       "10M",
			LogFilePurgeCount: 5,
		},
	}

	cm := BuildContainerSensorConfigMap("test-cm", "test-ns", platformConfig, sensorConfig)

	if cm.Name != "test-cm" {
		t.Errorf("expected name 'test-cm', got '%s'", cm.Name)
	}
	if cm.Namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got '%s'", cm.Namespace)
	}
	if cm.Data["SERVER_URI"] != "https://test.qualys.com/ContainerSensor" {
		t.Errorf("expected SERVER_URI 'https://test.qualys.com/ContainerSensor', got '%s'", cm.Data["SERVER_URI"])
	}
	if cm.Data["MODE"] != "general" {
		t.Errorf("expected MODE 'general', got '%s'", cm.Data["MODE"])
	}
	if cm.Data["K8S_MODE"] != "true" {
		t.Errorf("expected K8S_MODE 'true', got '%s'", cm.Data["K8S_MODE"])
	}
	if cm.Data["ENABLE_IMAGE_SCAN"] != "true" {
		t.Errorf("expected ENABLE_IMAGE_SCAN 'true', got '%s'", cm.Data["ENABLE_IMAGE_SCAN"])
	}
	if cm.Data["ENABLE_CONTAINER_SCAN"] != "true" {
		t.Errorf("expected ENABLE_CONTAINER_SCAN 'true', got '%s'", cm.Data["ENABLE_CONTAINER_SCAN"])
	}
	if cm.Data["SCAN_THREAD_POOL_SIZE"] != "4" {
		t.Errorf("expected SCAN_THREAD_POOL_SIZE '4', got '%s'", cm.Data["SCAN_THREAD_POOL_SIZE"])
	}
	if cm.Data["LOG_LEVEL"] != "3" {
		t.Errorf("expected LOG_LEVEL '3', got '%s'", cm.Data["LOG_LEVEL"])
	}
	if cm.Data["ENABLE_CONSOLE_LOGS"] != "true" {
		t.Errorf("expected ENABLE_CONSOLE_LOGS 'true', got '%s'", cm.Data["ENABLE_CONSOLE_LOGS"])
	}
}

func TestBuildContainerSensorConfigMapMinimal(t *testing.T) {
	platformConfig := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Platform: qualysv1.PlatformSettings{
				ServerUri: "https://test.qualys.com/ContainerSensor",
			},
		},
	}

	sensorConfig := qualysv1.ContainerSensorConfig{
		Mode: qualysv1.ContainerSensorModeGeneral,
	}

	cm := BuildContainerSensorConfigMap("test-cm", "test-ns", platformConfig, sensorConfig)

	if cm.Data["SERVER_URI"] != "https://test.qualys.com/ContainerSensor" {
		t.Errorf("expected SERVER_URI to be set")
	}
	if cm.Data["MODE"] != "general" {
		t.Errorf("expected MODE 'general', got '%s'", cm.Data["MODE"])
	}
	if _, ok := cm.Data["K8S_MODE"]; ok {
		t.Error("expected K8S_MODE to not be set when false")
	}
}

func TestBuildContainerSensorConfigMapWithMalwareAndSecret(t *testing.T) {
	platformConfig := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Platform: qualysv1.PlatformSettings{
				ServerUri: "https://test.qualys.com/ContainerSensor",
			},
		},
	}

	sensorConfig := qualysv1.ContainerSensorConfig{
		Mode: qualysv1.ContainerSensorModeGeneral,
		Scanning: &qualysv1.ScanningConfig{
			EnableMalwareDetection: true,
			EnableSecretDetection:  true,
			ContainerLaunchTimeout: "60s",
		},
	}

	cm := BuildContainerSensorConfigMap("test-cm", "test-ns", platformConfig, sensorConfig)

	if cm.Data["ENABLE_MALWARE_DETECTION"] != "true" {
		t.Errorf("expected ENABLE_MALWARE_DETECTION 'true', got '%s'", cm.Data["ENABLE_MALWARE_DETECTION"])
	}
	if cm.Data["ENABLE_SECRET_DETECTION"] != "true" {
		t.Errorf("expected ENABLE_SECRET_DETECTION 'true', got '%s'", cm.Data["ENABLE_SECRET_DETECTION"])
	}
	if cm.Data["CONTAINER_LAUNCH_TIMEOUT"] != "60s" {
		t.Errorf("expected CONTAINER_LAUNCH_TIMEOUT '60s', got '%s'", cm.Data["CONTAINER_LAUNCH_TIMEOUT"])
	}
}
