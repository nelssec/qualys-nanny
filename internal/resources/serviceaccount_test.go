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
)

func TestBuildServiceAccount(t *testing.T) {
	sa := BuildServiceAccount("test-sa", "test-ns", "container-sensor")

	if sa.Name != "test-sa" {
		t.Errorf("expected name 'test-sa', got '%s'", sa.Name)
	}
	if sa.Namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got '%s'", sa.Namespace)
	}
	if sa.Labels["app.kubernetes.io/component"] != "container-sensor" {
		t.Errorf("expected component label 'container-sensor', got '%s'", sa.Labels["app.kubernetes.io/component"])
	}
	if sa.Labels["app.kubernetes.io/managed-by"] != "qualys-nanny" {
		t.Errorf("expected managed-by label 'qualys-nanny', got '%s'", sa.Labels["app.kubernetes.io/managed-by"])
	}
}

func TestBuildContainerSensorClusterRole(t *testing.T) {
	role := BuildContainerSensorClusterRole("test-role")

	if role.Name != "test-role" {
		t.Errorf("expected name 'test-role', got '%s'", role.Name)
	}
	if len(role.Rules) == 0 {
		t.Error("expected rules to be non-empty")
	}
	if role.Labels["app.kubernetes.io/name"] != "qualys-container-sensor" {
		t.Errorf("expected name label 'qualys-container-sensor', got '%s'", role.Labels["app.kubernetes.io/name"])
	}
}

func TestBuildContainerSensorClusterRoleBinding(t *testing.T) {
	binding := BuildContainerSensorClusterRoleBinding("test-binding", "test-ns", "test-sa", "test-role")

	if binding.Name != "test-binding" {
		t.Errorf("expected name 'test-binding', got '%s'", binding.Name)
	}
	if binding.RoleRef.Name != "test-role" {
		t.Errorf("expected roleRef name 'test-role', got '%s'", binding.RoleRef.Name)
	}
	if binding.RoleRef.Kind != "ClusterRole" {
		t.Errorf("expected roleRef kind 'ClusterRole', got '%s'", binding.RoleRef.Kind)
	}
	if len(binding.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(binding.Subjects))
	}
	if binding.Subjects[0].Name != "test-sa" {
		t.Errorf("expected subject name 'test-sa', got '%s'", binding.Subjects[0].Name)
	}
	if binding.Subjects[0].Namespace != "test-ns" {
		t.Errorf("expected subject namespace 'test-ns', got '%s'", binding.Subjects[0].Namespace)
	}
}
