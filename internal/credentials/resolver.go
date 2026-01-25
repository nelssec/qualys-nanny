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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
)

const (
	ActivationIDKey = "ACTIVATION_ID"
	CustomerIDKey   = "CUSTOMER_ID"
)

type Credentials struct {
	ActivationID string
	CustomerID   string
	Source       string
}

type Resolver struct {
	client client.Client
}

func NewResolver(c client.Client) *Resolver {
	return &Resolver{client: c}
}

func (r *Resolver) Resolve(ctx context.Context, config *qualysv1.QualysPlatformConfig) (*Credentials, error) {
	switch config.Spec.Credentials.SourceType {
	case qualysv1.CredentialSourceSecret:
		return r.resolveFromSecret(ctx, config.Spec.Credentials.SecretRef)
	case qualysv1.CredentialSourceExternalSecret:
		return r.resolveFromExternalSecret(ctx, config.Spec.Credentials.ExternalSecretRef)
	default:
		return nil, fmt.Errorf("unknown credential source type: %s", config.Spec.Credentials.SourceType)
	}
}

func (r *Resolver) resolveFromSecret(ctx context.Context, ref *qualysv1.SecretReference) (*Credentials, error) {
	if ref == nil {
		return nil, fmt.Errorf("secret reference is nil")
	}

	secret := &corev1.Secret{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	activationID, ok := secret.Data[ActivationIDKey]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s missing required key: %s", ref.Namespace, ref.Name, ActivationIDKey)
	}

	customerID, ok := secret.Data[CustomerIDKey]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s missing required key: %s", ref.Namespace, ref.Name, CustomerIDKey)
	}

	return &Credentials{
		ActivationID: string(activationID),
		CustomerID:   string(customerID),
		Source:       fmt.Sprintf("secret/%s/%s", ref.Namespace, ref.Name),
	}, nil
}

func (r *Resolver) resolveFromExternalSecret(ctx context.Context, ref *qualysv1.ExternalSecretReference) (*Credentials, error) {
	if ref == nil {
		return nil, fmt.Errorf("external secret reference is nil")
	}

	secret := &corev1.Secret{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get synced secret %s/%s (from ExternalSecret): %w", ref.Namespace, ref.Name, err)
	}

	activationID, ok := secret.Data[ActivationIDKey]
	if !ok {
		return nil, fmt.Errorf("synced secret %s/%s missing required key: %s", ref.Namespace, ref.Name, ActivationIDKey)
	}

	customerID, ok := secret.Data[CustomerIDKey]
	if !ok {
		return nil, fmt.Errorf("synced secret %s/%s missing required key: %s", ref.Namespace, ref.Name, CustomerIDKey)
	}

	return &Credentials{
		ActivationID: string(activationID),
		CustomerID:   string(customerID),
		Source:       fmt.Sprintf("externalSecret/%s/%s", ref.Namespace, ref.Name),
	}, nil
}

func (r *Resolver) GetSecretRef(config *qualysv1.QualysPlatformConfig) (*qualysv1.SecretReference, error) {
	switch config.Spec.Credentials.SourceType {
	case qualysv1.CredentialSourceSecret:
		if config.Spec.Credentials.SecretRef == nil {
			return nil, fmt.Errorf("secret reference is nil")
		}
		return config.Spec.Credentials.SecretRef, nil
	case qualysv1.CredentialSourceExternalSecret:
		if config.Spec.Credentials.ExternalSecretRef == nil {
			return nil, fmt.Errorf("external secret reference is nil")
		}
		return &qualysv1.SecretReference{
			Name:      config.Spec.Credentials.ExternalSecretRef.Name,
			Namespace: config.Spec.Credentials.ExternalSecretRef.Namespace,
		}, nil
	default:
		return nil, fmt.Errorf("unknown credential source type: %s", config.Spec.Credentials.SourceType)
	}
}
