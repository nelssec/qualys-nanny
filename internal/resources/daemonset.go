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
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	qualysv1alpha1 "github.com/nelssec/qualys-nanny/api/v1alpha1"
)

const (
	CloudAgentContainerName      = "qualys-agent-installer"
	ContainerSensorContainerName = "qualys-container-sensor"
)

func BuildCloudAgentDaemonSet(
	agent *qualysv1alpha1.QualysCloudAgent,
	configMapName string,
	secretName string,
	serviceAccountName string,
) *appsv1.DaemonSet {
	image := agent.Spec.GetImage()
	scheduling := agent.Spec.GetScheduling()
	resources := agent.Spec.GetResources()
	updateStrategy := agent.Spec.GetUpdateStrategy()
	config := agent.Spec.GetConfig()

	labels := map[string]string{
		"app.kubernetes.io/name":       "qualys-cloud-agent",
		"app.kubernetes.io/instance":   agent.Name,
		"app.kubernetes.io/managed-by": "qualys-nanny",
		"app.kubernetes.io/component":  "cloud-agent",
	}

	envVars := buildCloudAgentEnvVars(configMapName, secretName, config)
	volumeMounts := buildCloudAgentVolumeMounts()
	volumes := buildCloudAgentVolumes()
	securityContext := buildPrivilegedSecurityContext()
	maxUnavailable := intstr.FromString(updateStrategy.RollingUpdate.MaxUnavailable)

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agent.Name,
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.DaemonSetUpdateStrategyType(updateStrategy.Type),
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            serviceAccountName,
					AutomountServiceAccountToken:  boolPtr(false),
					HostPID:                       true,
					HostNetwork:                   false,
					PriorityClassName:             scheduling.PriorityClassName,
					NodeSelector:                  scheduling.NodeSelector,
					Tolerations:                   scheduling.Tolerations,
					Affinity:                      scheduling.Affinity,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: int64Ptr(30),
					Containers: []corev1.Container{
						{
							Name:            CloudAgentContainerName,
							Image:           fmt.Sprintf("%s:%s", image.Repository, image.Tag),
							ImagePullPolicy: image.PullPolicy,
							SecurityContext: securityContext,
							Env:             envVars,
							VolumeMounts:    volumeMounts,
							Resources:       resources,
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/bash",
											"-c",
											"test -S /host/run/systemd/private && cat /host/proc/1/comm >/dev/null 2>&1",
										},
									},
								},
								InitialDelaySeconds: 120,
								PeriodSeconds:       60,
								TimeoutSeconds:      10,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/bash",
											"-c",
											"test -f /host/etc/qualys/cloud-agent/qualys-cloud-agent.conf",
										},
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       30,
								TimeoutSeconds:      10,
								FailureThreshold:    5,
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/bash",
											"-c",
											"nsenter --target 1 --mount --uts --ipc --net --pid -- systemctl stop qualys-cloud-agent 2>/dev/null || true",
										},
									},
								},
							},
						},
					},
					Volumes:          volumes,
					ImagePullSecrets: image.PullSecrets,
				},
			},
		},
	}

	return ds
}

func buildCloudAgentEnvVars(configMapName, secretName string, config qualysv1alpha1.CloudAgentConfig) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name: "ACTIVATION_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "ACTIVATION_ID",
				},
			},
		},
		{
			Name: "CUSTOMER_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "CUSTOMER_ID",
				},
			},
		},
		{
			Name: "SERVER_URI",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
					Key:                  "SERVER_URI",
				},
			},
		},
		{
			Name:  "LOG_LEVEL",
			Value: strconv.Itoa(config.LogLevel),
		},
		{
			Name: "NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}

	if config.LogFileDir != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "LOG_FILE_DIR",
			Value: config.LogFileDir,
		})
	}

	if config.LogDestType != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "LOG_DEST_TYPE",
			Value: config.LogDestType,
		})
	}

	if config.LogCompression {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "LOG_COMPRESSION",
			Value: "1",
		})
	}

	if config.CmdMaxTimeOut > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "CMD_MAX_TIMEOUT",
			Value: strconv.Itoa(config.CmdMaxTimeOut),
		})
	}

	if config.ProcessPriority != 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PROCESS_PRIORITY",
			Value: strconv.Itoa(config.ProcessPriority),
		})
	}

	if config.ScanDelayVM != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SCAN_DELAY_VM",
			Value: strconv.Itoa(*config.ScanDelayVM),
		})
	}

	if config.ScanDelayPC != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SCAN_DELAY_PC",
			Value: strconv.Itoa(*config.ScanDelayPC),
		})
	}

	if config.UseSudo {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "USE_SUDO",
			Value: "1",
		})
	}

	if config.SudoCommand != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "SUDO_COMMAND",
			Value: config.SudoCommand,
		})
	}

	if config.User != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "AGENT_USER",
			Value: config.User,
		})
	}

	if config.Group != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "AGENT_GROUP",
			Value: config.Group,
		})
	}

	if config.UseAuditDispatcher {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "USE_AUDIT_DISPATCHER",
			Value: "1",
		})
	}

	if config.HostIdSearchDir != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "HOST_ID_SEARCH_DIR",
			Value: config.HostIdSearchDir,
		})
	}

	return envVars
}

func buildCloudAgentVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "host-tmp", MountPath: "/host/tmp"},
		{Name: "host-etc", MountPath: "/host/etc"},
		{Name: "host-var", MountPath: "/host/var"},
		{Name: "host-usr", MountPath: "/host/usr"},
		{Name: "host-opt", MountPath: "/host/opt"},
		{Name: "host-lib", MountPath: "/host/lib"},
		{Name: "host-lib64", MountPath: "/host/lib64"},
		{Name: "host-run", MountPath: "/host/run"},
		{Name: "host-proc", MountPath: "/host/proc", ReadOnly: true},
		{Name: "host-sys", MountPath: "/host/sys", ReadOnly: true},
		{Name: "host-bin", MountPath: "/host/bin", ReadOnly: true},
		{Name: "host-sbin", MountPath: "/host/sbin", ReadOnly: true},
	}
}

func buildCloudAgentVolumes() []corev1.Volume {
	directory := corev1.HostPathDirectory
	directoryOrCreate := corev1.HostPathDirectoryOrCreate

	return []corev1.Volume{
		{Name: "host-tmp", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/tmp", Type: &directory}}},
		{Name: "host-etc", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc", Type: &directory}}},
		{Name: "host-var", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var", Type: &directory}}},
		{Name: "host-usr", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/usr", Type: &directory}}},
		{Name: "host-opt", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/opt", Type: &directoryOrCreate}}},
		{Name: "host-lib", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/lib", Type: &directory}}},
		{Name: "host-lib64", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/lib64", Type: &directoryOrCreate}}},
		{Name: "host-run", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/run", Type: &directory}}},
		{Name: "host-proc", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/proc", Type: &directory}}},
		{Name: "host-sys", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sys", Type: &directory}}},
		{Name: "host-bin", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/bin", Type: &directory}}},
		{Name: "host-sbin", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/sbin", Type: &directory}}},
	}
}

func buildPrivilegedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               boolPtr(true),
		RunAsUser:                int64Ptr(0),
		RunAsNonRoot:             boolPtr(false),
		AllowPrivilegeEscalation: boolPtr(true),
		ReadOnlyRootFilesystem:   boolPtr(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeUnconfined,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
			Add: []corev1.Capability{
				"SYS_ADMIN",
				"SYS_CHROOT",
				"SYS_PTRACE",
			},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func int64Ptr(i int64) *int64 {
	return &i
}
