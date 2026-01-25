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

func TestBuildContainerSensorSCC(t *testing.T) {
	scc := BuildContainerSensorSCC("test-scc", "test-ns", "test-sa")

	if scc.GetName() != "test-scc" {
		t.Errorf("expected name 'test-scc', got '%s'", scc.GetName())
	}
	if scc.GetKind() != "SecurityContextConstraints" {
		t.Errorf("expected kind 'SecurityContextConstraints', got '%s'", scc.GetKind())
	}
	users, found, _ := getStringSlice(scc.Object, "users")
	if !found || len(users) == 0 {
		t.Error("expected users to be set")
	}
	expectedUser := "system:serviceaccount:test-ns:test-sa"
	if users[0] != expectedUser {
		t.Errorf("expected user '%s', got '%s'", expectedUser, users[0])
	}
}

func TestBuildContainerSensorSCCWithPrivilegeMode(t *testing.T) {
	tests := []struct {
		name          string
		privilegeMode qualysv1.PrivilegeMode
		expectPriv    bool
		expectHostPID bool
	}{
		{
			name:          "unprivileged",
			privilegeMode: qualysv1.PrivilegeModeUnprivileged,
			expectPriv:    false,
			expectHostPID: false,
		},
		{
			name:          "minimal",
			privilegeMode: qualysv1.PrivilegeModeMinimal,
			expectPriv:    false,
			expectHostPID: true,
		},
		{
			name:          "standard",
			privilegeMode: qualysv1.PrivilegeModeStandard,
			expectPriv:    false,
			expectHostPID: true,
		},
		{
			name:          "privileged",
			privilegeMode: qualysv1.PrivilegeModePrivileged,
			expectPriv:    false,
			expectHostPID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scc := BuildContainerSensorSCCWithPrivilegeMode("test-scc", "test-ns", "test-sa", tt.privilegeMode)

			if scc.GetName() != "test-scc" {
				t.Errorf("expected name 'test-scc', got '%s'", scc.GetName())
			}

			allowPriv, _ := scc.Object["allowPrivilegedContainer"].(bool)
			if allowPriv != tt.expectPriv {
				t.Errorf("expected allowPrivilegedContainer=%v, got %v", tt.expectPriv, allowPriv)
			}

			allowHostPID, _ := scc.Object["allowHostPID"].(bool)
			if allowHostPID != tt.expectHostPID {
				t.Errorf("expected allowHostPID=%v, got %v", tt.expectHostPID, allowHostPID)
			}
		})
	}
}

func TestBuildRuntimeSensorSCC(t *testing.T) {
	scc := BuildRuntimeSensorSCC("runtime-scc", "test-ns", "test-sa")

	if scc.GetName() != "runtime-scc" {
		t.Errorf("expected name 'runtime-scc', got '%s'", scc.GetName())
	}

	allowPriv, ok := scc.Object["allowPrivilegedContainer"].(bool)
	if !ok || !allowPriv {
		t.Error("expected allowPrivilegedContainer=true for runtime sensor")
	}

	allowHostPID, ok := scc.Object["allowHostPID"].(bool)
	if !ok || !allowHostPID {
		t.Error("expected allowHostPID=true for runtime sensor")
	}

	allowHostNetwork, ok := scc.Object["allowHostNetwork"].(bool)
	if !ok || !allowHostNetwork {
		t.Error("expected allowHostNetwork=true for runtime sensor")
	}
}

func TestBuildClusterSensorSCC(t *testing.T) {
	scc := BuildClusterSensorSCC("cluster-scc", "test-ns", "test-sa")

	if scc.GetName() != "cluster-scc" {
		t.Errorf("expected name 'cluster-scc', got '%s'", scc.GetName())
	}

	allowPriv, ok := scc.Object["allowPrivilegedContainer"].(bool)
	if ok && allowPriv {
		t.Error("expected allowPrivilegedContainer=false for cluster sensor")
	}

	allowHostNetwork, ok := scc.Object["allowHostNetwork"].(bool)
	if !ok || !allowHostNetwork {
		t.Error("expected allowHostNetwork=true for cluster sensor")
	}
}

func getStringSlice(obj map[string]interface{}, key string) ([]string, bool, error) {
	val, found := obj[key]
	if !found {
		return nil, false, nil
	}
	slice, ok := val.([]interface{})
	if !ok {
		return nil, true, nil
	}
	result := make([]string, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			return nil, true, nil
		}
		result[i] = s
	}
	return result, true, nil
}
