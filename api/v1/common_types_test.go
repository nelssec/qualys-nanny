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

package v1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetConditionNew(t *testing.T) {
	conditions := []metav1.Condition{}

	condition := metav1.Condition{
		Type:    ConditionTypeAvailable,
		Status:  metav1.ConditionTrue,
		Reason:  "AllComponentsReady",
		Message: "All components are ready",
	}

	SetCondition(&conditions, condition)

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].Type != ConditionTypeAvailable {
		t.Errorf("expected type '%s', got '%s'", ConditionTypeAvailable, conditions[0].Type)
	}
	if conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %s", conditions[0].Status)
	}
	if conditions[0].LastTransitionTime.IsZero() {
		t.Error("expected LastTransitionTime to be set")
	}
}

func TestSetConditionUpdate(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:               ConditionTypeAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "NotReady",
			Message:            "Components not ready",
			LastTransitionTime: metav1.Now(),
		},
	}

	condition := metav1.Condition{
		Type:    ConditionTypeAvailable,
		Status:  metav1.ConditionTrue,
		Reason:  "AllComponentsReady",
		Message: "All components are ready",
	}

	SetCondition(&conditions, condition)

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %s", conditions[0].Status)
	}
	if conditions[0].Reason != "AllComponentsReady" {
		t.Errorf("expected reason 'AllComponentsReady', got '%s'", conditions[0].Reason)
	}
}

func TestSetConditionNoChange(t *testing.T) {
	originalTime := metav1.Now()
	conditions := []metav1.Condition{
		{
			Type:               ConditionTypeAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "AllComponentsReady",
			Message:            "All components are ready",
			LastTransitionTime: originalTime,
		},
	}

	condition := metav1.Condition{
		Type:    ConditionTypeAvailable,
		Status:  metav1.ConditionTrue,
		Reason:  "AllComponentsReady",
		Message: "All components are ready",
	}

	SetCondition(&conditions, condition)

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].LastTransitionTime != originalTime {
		t.Error("expected LastTransitionTime to remain unchanged when condition didn't change")
	}
}

func TestSetConditionMultiple(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:               ConditionTypeAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Ready",
			Message:            "Ready",
			LastTransitionTime: metav1.Now(),
		},
	}

	condition := metav1.Condition{
		Type:    ConditionTypeProgressing,
		Status:  metav1.ConditionFalse,
		Reason:  "NotProgressing",
		Message: "Not progressing",
	}

	SetCondition(&conditions, condition)

	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}
}

func TestGetConditionFound(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:    ConditionTypeAvailable,
			Status:  metav1.ConditionTrue,
			Reason:  "Ready",
			Message: "Ready",
		},
		{
			Type:    ConditionTypeProgressing,
			Status:  metav1.ConditionFalse,
			Reason:  "NotProgressing",
			Message: "Not progressing",
		},
	}

	result := GetCondition(conditions, ConditionTypeAvailable)

	if result == nil {
		t.Fatal("expected condition to be found")
	}
	if result.Type != ConditionTypeAvailable {
		t.Errorf("expected type '%s', got '%s'", ConditionTypeAvailable, result.Type)
	}
}

func TestGetConditionNotFound(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:    ConditionTypeAvailable,
			Status:  metav1.ConditionTrue,
			Reason:  "Ready",
			Message: "Ready",
		},
	}

	result := GetCondition(conditions, ConditionTypeDegraded)

	if result != nil {
		t.Error("expected nil for non-existent condition")
	}
}

func TestGetConditionEmpty(t *testing.T) {
	conditions := []metav1.Condition{}

	result := GetCondition(conditions, ConditionTypeAvailable)

	if result != nil {
		t.Error("expected nil for empty conditions slice")
	}
}

func TestCredentialSourceTypes(t *testing.T) {
	if CredentialSourceSecret != "secret" {
		t.Errorf("expected 'secret', got '%s'", CredentialSourceSecret)
	}
	if CredentialSourceExternalSecret != "externalSecret" {
		t.Errorf("expected 'externalSecret', got '%s'", CredentialSourceExternalSecret)
	}
}

func TestConditionTypes(t *testing.T) {
	if ConditionTypeAvailable != "Available" {
		t.Errorf("expected 'Available', got '%s'", ConditionTypeAvailable)
	}
	if ConditionTypeProgressing != "Progressing" {
		t.Errorf("expected 'Progressing', got '%s'", ConditionTypeProgressing)
	}
	if ConditionTypeDegraded != "Degraded" {
		t.Errorf("expected 'Degraded', got '%s'", ConditionTypeDegraded)
	}
	if ConditionTypeCredentialsReady != "CredentialsReady" {
		t.Errorf("expected 'CredentialsReady', got '%s'", ConditionTypeCredentialsReady)
	}
	if ConditionTypePlatformReachable != "PlatformReachable" {
		t.Errorf("expected 'PlatformReachable', got '%s'", ConditionTypePlatformReachable)
	}
}
