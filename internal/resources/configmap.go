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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qualysv1alpha1 "github.com/nelssec/qualys-nanny/api/v1alpha1"
)

const boolTrue = "true"

func BuildCloudAgentConfigMap(name, namespace string, platformConfig *qualysv1alpha1.QualysPlatformConfig, agentConfig qualysv1alpha1.CloudAgentConfig) *corev1.ConfigMap {
	data := map[string]string{
		"SERVER_URI": platformConfig.Spec.Platform.ServerUri,
		"LOG_LEVEL":  strconv.Itoa(agentConfig.LogLevel),
	}

	if agentConfig.LogFileDir != "" {
		data["LOG_FILE_DIR"] = agentConfig.LogFileDir
	}

	if agentConfig.LogDestType != "" {
		data["LOG_DEST_TYPE"] = agentConfig.LogDestType
	}

	if agentConfig.LogCompression {
		data["LOG_COMPRESSION"] = "1"
	}

	if agentConfig.CmdMaxTimeOut > 0 {
		data["CMD_MAX_TIMEOUT"] = strconv.Itoa(agentConfig.CmdMaxTimeOut)
	}

	if agentConfig.ProcessPriority != 0 {
		data["PROCESS_PRIORITY"] = strconv.Itoa(agentConfig.ProcessPriority)
	}

	if agentConfig.ScanDelayVM != nil {
		data["SCAN_DELAY_VM"] = strconv.Itoa(*agentConfig.ScanDelayVM)
	}

	if agentConfig.ScanDelayPC != nil {
		data["SCAN_DELAY_PC"] = strconv.Itoa(*agentConfig.ScanDelayPC)
	}

	if agentConfig.MaxRandomScanIntervalVM != nil {
		data["MAX_RANDOM_SCAN_INTERVAL_VM"] = strconv.Itoa(*agentConfig.MaxRandomScanIntervalVM)
	}

	if agentConfig.MaxRandomScanIntervalPC != nil {
		data["MAX_RANDOM_SCAN_INTERVAL_PC"] = strconv.Itoa(*agentConfig.MaxRandomScanIntervalPC)
	}

	if agentConfig.UseSudo {
		data["USE_SUDO"] = "1"
	}

	if agentConfig.SudoCommand != "" {
		data["SUDO_COMMAND"] = agentConfig.SudoCommand
	}

	if agentConfig.User != "" {
		data["AGENT_USER"] = agentConfig.User
	}

	if agentConfig.Group != "" {
		data["AGENT_GROUP"] = agentConfig.Group
	}

	if agentConfig.UseAuditDispatcher {
		data["USE_AUDIT_DISPATCHER"] = "1"
	}

	if agentConfig.HostIdSearchDir != "" {
		data["HOST_ID_SEARCH_DIR"] = agentConfig.HostIdSearchDir
	}

	if platformConfig.Spec.Platform.Proxy != nil {
		proxy := platformConfig.Spec.Platform.Proxy
		if proxy.ProxyOrder != "" {
			data["QUALYS_PROXY_ORDER"] = proxy.ProxyOrder
		}
		if proxy.ProxyFailOpen {
			data["PROXY_FAIL_OPEN"] = "1"
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-cloud-agent",
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "config",
			},
		},
		Data: data,
	}
}

func BuildContainerSensorConfigMap(name, namespace string, platformConfig *qualysv1alpha1.QualysPlatformConfig, sensorConfig qualysv1alpha1.ContainerSensorConfig) *corev1.ConfigMap {
	data := map[string]string{
		"SERVER_URI": platformConfig.Spec.Platform.ServerUri,
		"MODE":       string(sensorConfig.Mode),
	}

	if sensorConfig.K8sMode {
		data["K8S_MODE"] = boolTrue
	}

	if sensorConfig.Scanning != nil {
		if sensorConfig.Scanning.EnableImageScan {
			data["ENABLE_IMAGE_SCAN"] = boolTrue
		}
		if sensorConfig.Scanning.EnableContainerScan {
			data["ENABLE_CONTAINER_SCAN"] = boolTrue
		}
		if sensorConfig.Scanning.EnableMalwareDetection {
			data["ENABLE_MALWARE_DETECTION"] = boolTrue
		}
		if sensorConfig.Scanning.EnableSecretDetection {
			data["ENABLE_SECRET_DETECTION"] = boolTrue
		}
		if sensorConfig.Scanning.ScanThreadPoolSize > 0 {
			data["SCAN_THREAD_POOL_SIZE"] = strconv.Itoa(sensorConfig.Scanning.ScanThreadPoolSize)
		}
		if sensorConfig.Scanning.ContainerLaunchTimeout != "" {
			data["CONTAINER_LAUNCH_TIMEOUT"] = sensorConfig.Scanning.ContainerLaunchTimeout
		}
	}

	if sensorConfig.Logging != nil {
		if sensorConfig.Logging.EnableConsoleLogs {
			data["ENABLE_CONSOLE_LOGS"] = boolTrue
		}
		data["LOG_LEVEL"] = strconv.Itoa(sensorConfig.Logging.LogLevel)
		if sensorConfig.Logging.LogFileSize != "" {
			data["LOG_FILE_SIZE"] = sensorConfig.Logging.LogFileSize
		}
		if sensorConfig.Logging.LogFilePurgeCount > 0 {
			data["LOG_FILE_PURGE_COUNT"] = strconv.Itoa(sensorConfig.Logging.LogFilePurgeCount)
		}
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-container-sensor",
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "config",
			},
		},
		Data: data,
	}
}
