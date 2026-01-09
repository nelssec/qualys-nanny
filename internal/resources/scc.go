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
)

var SCCGVK = schema.GroupVersionKind{
	Group:   "security.openshift.io",
	Version: "v1",
	Kind:    "SecurityContextConstraints",
}

func BuildCloudAgentSCC(name, namespace, serviceAccountName string) *unstructured.Unstructured {
	scc := &unstructured.Unstructured{}
	scc.SetGroupVersionKind(SCCGVK)
	scc.SetName(name)
	scc.SetAnnotations(map[string]string{
		"kubernetes.io/description": "SCC for Qualys Cloud Agent - requires host access for system scanning",
	})

	scc.Object["allowHostDirVolumePlugin"] = true
	scc.Object["allowHostIPC"] = false
	scc.Object["allowHostNetwork"] = false
	scc.Object["allowHostPID"] = true
	scc.Object["allowHostPorts"] = false
	scc.Object["allowPrivilegeEscalation"] = true
	scc.Object["allowPrivilegedContainer"] = true
	scc.Object["allowedCapabilities"] = []interface{}{
		"SYS_PTRACE",
		"SYS_ADMIN",
		"SYS_CHROOT",
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

func BuildContainerSensorSCC(name, namespace, serviceAccountName string) *unstructured.Unstructured {
	scc := &unstructured.Unstructured{}
	scc.SetGroupVersionKind(SCCGVK)
	scc.SetName(name)
	scc.SetAnnotations(map[string]string{
		"kubernetes.io/description": "SCC for Qualys Container Security Sensor - requires privileged access for container scanning",
	})

	scc.Object["allowHostDirVolumePlugin"] = true
	scc.Object["allowHostIPC"] = false
	scc.Object["allowHostNetwork"] = true
	scc.Object["allowHostPID"] = true
	scc.Object["allowHostPorts"] = false
	scc.Object["allowPrivilegeEscalation"] = true
	scc.Object["allowPrivilegedContainer"] = true
	scc.Object["allowedCapabilities"] = []interface{}{
		"SYS_PTRACE",
		"SYS_ADMIN",
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
