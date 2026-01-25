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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
)

var SCCGVK = schema.GroupVersionKind{
	Group:   "security.openshift.io",
	Version: "v1",
	Kind:    "SecurityContextConstraints",
}

func BuildContainerSensorSCC(name, namespace, serviceAccountName string) *unstructured.Unstructured {
	return BuildContainerSensorSCCWithPrivilegeMode(name, namespace, serviceAccountName, qualysv1.PrivilegeModeStandard)
}

func BuildRuntimeSensorSCC(name, namespace, serviceAccountName string) *unstructured.Unstructured {
	scc := &unstructured.Unstructured{}
	scc.SetGroupVersionKind(SCCGVK)
	scc.SetName(name)
	scc.SetAnnotations(map[string]string{
		"kubernetes.io/description": "SCC for Qualys Runtime Sensor (eBPF) - requires privileged mode for kernel tracing",
	})

	scc.Object["allowHostDirVolumePlugin"] = true
	scc.Object["allowHostIPC"] = false
	scc.Object["allowHostNetwork"] = true
	scc.Object["allowHostPID"] = true
	scc.Object["allowHostPorts"] = false
	scc.Object["allowPrivilegeEscalation"] = true
	scc.Object["allowPrivilegedContainer"] = true
	scc.Object["allowedCapabilities"] = []interface{}{"*"}
	scc.Object["defaultAddCapabilities"] = []interface{}{}
	scc.Object["fsGroup"] = map[string]interface{}{
		"type": "RunAsAny",
	}
	scc.Object["groups"] = []interface{}{}
	scc.Object["priority"] = nil
	scc.Object["readOnlyRootFilesystem"] = false
	scc.Object["requiredDropCapabilities"] = []interface{}{}
	scc.Object["runAsUser"] = map[string]interface{}{
		"type": "RunAsAny",
	}
	scc.Object["seLinuxContext"] = map[string]interface{}{
		"type": "RunAsAny",
	}
	scc.Object["supplementalGroups"] = map[string]interface{}{
		"type": "RunAsAny",
	}
	scc.Object["users"] = []interface{}{
		"system:serviceaccount:" + namespace + ":" + serviceAccountName,
	}
	scc.Object["volumes"] = []interface{}{
		"configMap",
		"downwardAPI",
		"emptyDir",
		"hostPath",
		"persistentVolumeClaim",
		"projected",
		"secret",
	}

	return scc
}

func BuildClusterSensorSCC(name, namespace, serviceAccountName string) *unstructured.Unstructured {
	scc := &unstructured.Unstructured{}
	scc.SetGroupVersionKind(SCCGVK)
	scc.SetName(name)

	scc.Object["allowHostDirVolumePlugin"] = true
	scc.Object["allowHostIPC"] = false
	scc.Object["allowHostNetwork"] = true
	scc.Object["allowHostPID"] = false
	scc.Object["allowHostPorts"] = false
	scc.Object["allowPrivilegedContainer"] = false
	scc.Object["readOnlyRootFilesystem"] = false
	scc.Object["runAsUser"] = map[string]interface{}{
		"type": "RunAsAny",
	}
	scc.Object["seLinuxContext"] = map[string]interface{}{
		"type": "RunAsAny",
	}
	scc.Object["users"] = []interface{}{
		"system:serviceaccount:" + namespace + ":" + serviceAccountName,
	}

	return scc
}

func BuildContainerSensorSCCWithPrivilegeMode(name, namespace, serviceAccountName string, privilegeMode qualysv1.PrivilegeMode) *unstructured.Unstructured {
	scc := &unstructured.Unstructured{}
	scc.SetGroupVersionKind(SCCGVK)
	scc.SetName(name)

	switch privilegeMode {
	case qualysv1.PrivilegeModeUnprivileged:
		scc.SetAnnotations(map[string]string{
			"kubernetes.io/description": "SCC for Qualys Container Sensor (unprivileged) - image scanning via CRI API and static scanning with DAC_READ_SEARCH",
		})
		scc.Object["allowHostDirVolumePlugin"] = true
		scc.Object["allowHostIPC"] = false
		scc.Object["allowHostNetwork"] = false
		scc.Object["allowHostPID"] = false
		scc.Object["allowHostPorts"] = false
		scc.Object["allowPrivilegeEscalation"] = false
		scc.Object["allowPrivilegedContainer"] = false
		scc.Object["allowedCapabilities"] = []interface{}{"DAC_READ_SEARCH"}
		scc.Object["defaultAddCapabilities"] = []interface{}{}
		scc.Object["fsGroup"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["groups"] = []interface{}{}
		scc.Object["priority"] = nil
		scc.Object["readOnlyRootFilesystem"] = true
		scc.Object["requiredDropCapabilities"] = []interface{}{"ALL"}
		scc.Object["runAsUser"] = map[string]interface{}{
			"type": "MustRunAsNonRoot",
		}
		scc.Object["seLinuxContext"] = map[string]interface{}{
			"type": "MustRunAs",
		}
		scc.Object["supplementalGroups"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["seccompProfiles"] = []interface{}{
			"runtime/default",
		}
		scc.Object["volumes"] = []interface{}{
			"configMap",
			"downwardAPI",
			"emptyDir",
			"hostPath",
			"projected",
			"secret",
		}

	case qualysv1.PrivilegeModeMinimal:
		scc.SetAnnotations(map[string]string{
			"kubernetes.io/description": "SCC for Qualys Container Sensor (minimal) - image and container scanning with SYS_PTRACE",
		})
		scc.Object["allowHostDirVolumePlugin"] = true
		scc.Object["allowHostIPC"] = false
		scc.Object["allowHostNetwork"] = true
		scc.Object["allowHostPID"] = true
		scc.Object["allowHostPorts"] = false
		scc.Object["allowPrivilegeEscalation"] = false
		scc.Object["allowPrivilegedContainer"] = false
		scc.Object["allowedCapabilities"] = []interface{}{"SYS_PTRACE"}
		scc.Object["defaultAddCapabilities"] = []interface{}{}
		scc.Object["fsGroup"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["groups"] = []interface{}{}
		scc.Object["priority"] = nil
		scc.Object["readOnlyRootFilesystem"] = false
		scc.Object["requiredDropCapabilities"] = []interface{}{"ALL"}
		scc.Object["runAsUser"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["seLinuxContext"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["supplementalGroups"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["seccompProfiles"] = []interface{}{
			"runtime/default",
		}
		scc.Object["volumes"] = []interface{}{
			"configMap",
			"downwardAPI",
			"emptyDir",
			"hostPath",
			"persistentVolumeClaim",
			"projected",
			"secret",
		}

	default:
		scc.SetAnnotations(map[string]string{
			"kubernetes.io/description": "SCC for Qualys Container Sensor (standard) - full scanning with SYS_ADMIN, SYS_PTRACE, SYS_CHROOT",
		})
		scc.Object["allowHostDirVolumePlugin"] = true
		scc.Object["allowHostIPC"] = false
		scc.Object["allowHostNetwork"] = true
		scc.Object["allowHostPID"] = true
		scc.Object["allowHostPorts"] = false
		scc.Object["allowPrivilegeEscalation"] = true
		scc.Object["allowPrivilegedContainer"] = false
		scc.Object["allowedCapabilities"] = []interface{}{
			"SYS_ADMIN",
			"SYS_PTRACE",
			"SYS_CHROOT",
			"DAC_READ_SEARCH",
		}
		scc.Object["defaultAddCapabilities"] = []interface{}{}
		scc.Object["fsGroup"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["groups"] = []interface{}{}
		scc.Object["priority"] = nil
		scc.Object["readOnlyRootFilesystem"] = false
		scc.Object["requiredDropCapabilities"] = []interface{}{}
		scc.Object["runAsUser"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["seLinuxContext"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["supplementalGroups"] = map[string]interface{}{
			"type": "RunAsAny",
		}
		scc.Object["seccompProfiles"] = []interface{}{
			"runtime/default",
			"*",
		}
		scc.Object["volumes"] = []interface{}{
			"configMap",
			"downwardAPI",
			"emptyDir",
			"hostPath",
			"persistentVolumeClaim",
			"projected",
			"secret",
		}
	}

	scc.Object["users"] = []interface{}{
		"system:serviceaccount:" + namespace + ":" + serviceAccountName,
	}

	return scc
}
