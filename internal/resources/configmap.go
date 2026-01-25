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

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
)

const boolTrue = "true"

func BuildContainerSensorConfigMap(name, namespace string, platformConfig *qualysv1.QualysPlatformConfig, sensorConfig qualysv1.ContainerSensorConfig) *corev1.ConfigMap {
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

	if platformConfig.Spec.Platform.Proxy != nil {
		proxy := platformConfig.Spec.Platform.Proxy
		if proxy.QualysHttpsProxy != "" {
			data["QUALYS_HTTPS_PROXY"] = proxy.QualysHttpsProxy
		}
		if proxy.HttpsProxy != "" {
			data["HTTPS_PROXY"] = proxy.HttpsProxy
		}
		if proxy.ProxyOrder != "" {
			data["PROXY_ORDER"] = proxy.ProxyOrder
		}
		if proxy.ProxyFailOpen {
			data["PROXY_FAIL_OPEN"] = boolTrue
		}
		if proxy.CACertBundle != "" {
			data["CA_CERT_BUNDLE"] = proxy.CACertBundle
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
