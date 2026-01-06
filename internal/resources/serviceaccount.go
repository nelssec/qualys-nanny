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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildServiceAccount(name, namespace, component string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-" + component,
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  component,
			},
		},
	}
}

func BuildCloudAgentClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-cloud-agent",
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "cloud-agent",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "pods"},
				Verbs:     []string{"get", "list"},
			},
		},
	}
}

func BuildCloudAgentClusterRoleBinding(name, namespace, serviceAccountName, clusterRoleName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-cloud-agent",
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "cloud-agent",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}
}

func BuildContainerSensorClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-container-sensor",
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "container-sensor",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "pods", "namespaces", "services", "configmaps", "secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "replicasets", "daemonsets", "statefulsets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "cronjobs"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func BuildContainerSensorClusterRoleBinding(name, namespace, serviceAccountName, clusterRoleName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-container-sensor",
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "container-sensor",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}
}
