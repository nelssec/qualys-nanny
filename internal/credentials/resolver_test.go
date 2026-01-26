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

package credentials

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
)

func TestNewResolver(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	resolver := NewResolver(client)
	if resolver == nil {
		t.Fatal("expected resolver to not be nil")
	}
	if resolver.client == nil {
		t.Error("expected resolver client to not be nil")
	}
}

func TestResolveFromSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"ACTIVATION_ID": []byte("test-activation-id"),
			"CUSTOMER_ID":   []byte("test-customer-id"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef: &qualysv1.SecretReference{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	creds, err := resolver.Resolve(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ActivationID != "test-activation-id" {
		t.Errorf("expected activation ID 'test-activation-id', got '%s'", creds.ActivationID)
	}
	if creds.CustomerID != "test-customer-id" {
		t.Errorf("expected customer ID 'test-customer-id', got '%s'", creds.CustomerID)
	}
	if creds.Source != "secret/test-ns/test-secret" {
		t.Errorf("expected source 'secret/test-ns/test-secret', got '%s'", creds.Source)
	}
}

func TestResolveFromSecretMissingActivationID(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"CUSTOMER_ID": []byte("test-customer-id"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef: &qualysv1.SecretReference{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for missing ACTIVATION_ID")
	}
}

func TestResolveFromSecretMissingCustomerID(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"ACTIVATION_ID": []byte("test-activation-id"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef: &qualysv1.SecretReference{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for missing CUSTOMER_ID")
	}
}

func TestResolveFromSecretNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef: &qualysv1.SecretReference{
					Name:      "nonexistent",
					Namespace: "test-ns",
				},
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for nonexistent secret")
	}
}

func TestResolveFromSecretNilRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef:  nil,
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for nil secret ref")
	}
}

func TestResolveFromExternalSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-external-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"ACTIVATION_ID": []byte("ext-activation-id"),
			"CUSTOMER_ID":   []byte("ext-customer-id"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: &qualysv1.ExternalSecretReference{
					Name:      "test-external-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	creds, err := resolver.Resolve(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ActivationID != "ext-activation-id" {
		t.Errorf("expected activation ID 'ext-activation-id', got '%s'", creds.ActivationID)
	}
	if creds.Source != "externalSecret/test-ns/test-external-secret" {
		t.Errorf("expected source 'externalSecret/test-ns/test-external-secret', got '%s'", creds.Source)
	}
}

func TestResolveFromExternalSecretNilRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType:        qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: nil,
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for nil external secret ref")
	}
}

func TestResolveUnknownSourceType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: "unknown",
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for unknown source type")
	}
}

func TestGetSecretRefFromSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef: &qualysv1.SecretReference{
					Name:      "test-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	ref, err := resolver.GetSecretRef(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Name != "test-secret" {
		t.Errorf("expected name 'test-secret', got '%s'", ref.Name)
	}
	if ref.Namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got '%s'", ref.Namespace)
	}
}

func TestGetSecretRefFromExternalSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: &qualysv1.ExternalSecretReference{
					Name:      "test-external",
					Namespace: "test-ns",
				},
			},
		},
	}

	ref, err := resolver.GetSecretRef(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Name != "test-external" {
		t.Errorf("expected name 'test-external', got '%s'", ref.Name)
	}
}

func TestGetSecretRefNilSecretRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceSecret,
				SecretRef:  nil,
			},
		},
	}

	_, err := resolver.GetSecretRef(config)
	if err == nil {
		t.Error("expected error for nil secret ref")
	}
}

func TestGetSecretRefNilExternalSecretRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType:        qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: nil,
			},
		},
	}

	_, err := resolver.GetSecretRef(config)
	if err == nil {
		t.Error("expected error for nil external secret ref")
	}
}

func TestGetSecretRefUnknownType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: "unknown",
			},
		},
	}

	_, err := resolver.GetSecretRef(config)
	if err == nil {
		t.Error("expected error for unknown source type")
	}
}

func TestResolveFromExternalSecretMissingActivationID(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-external-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"CUSTOMER_ID": []byte("ext-customer-id"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: &qualysv1.ExternalSecretReference{
					Name:      "test-external-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for missing ACTIVATION_ID")
	}
}

func TestResolveFromExternalSecretMissingCustomerID(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-external-secret",
			Namespace: "test-ns",
		},
		Data: map[string][]byte{
			"ACTIVATION_ID": []byte("ext-activation-id"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: &qualysv1.ExternalSecretReference{
					Name:      "test-external-secret",
					Namespace: "test-ns",
				},
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for missing CUSTOMER_ID")
	}
}

func TestResolveFromExternalSecretNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	resolver := NewResolver(client)

	config := &qualysv1.QualysPlatformConfig{
		Spec: qualysv1.QualysPlatformConfigSpec{
			Credentials: qualysv1.CredentialsConfig{
				SourceType: qualysv1.CredentialSourceExternalSecret,
				ExternalSecretRef: &qualysv1.ExternalSecretReference{
					Name:      "nonexistent",
					Namespace: "test-ns",
				},
			},
		},
	}

	_, err := resolver.Resolve(context.Background(), config)
	if err == nil {
		t.Error("expected error for nonexistent secret")
	}
}
