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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
)

var _ = Describe("QualysPlatformConfig Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-platform-config"
			secretName   = "test-credentials"
			namespace    = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name: resourceName,
		}

		BeforeEach(func() {
			By("creating the credentials secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"ACTIVATION_ID": []byte("test-activation-id"),
					"CUSTOMER_ID":   []byte("test-customer-id"),
				},
			}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &corev1.Secret{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			}

			By("creating the custom resource for the Kind QualysPlatformConfig")
			resource := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: resourceName,
				},
				Spec: qualysv1.QualysPlatformConfigSpec{
					Platform: qualysv1.PlatformSettings{
						ServerUri: "https://cmsqagpublic.qg2.apps.qualys.com/ContainerSensor",
					},
					Credentials: qualysv1.CredentialsConfig{
						SourceType: qualysv1.CredentialSourceSecret,
						SecretRef: &qualysv1.SecretReference{
							Name:      secretName,
							Namespace: namespace,
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance QualysPlatformConfig")
			resource := &qualysv1.QualysPlatformConfig{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the credentials secret")
			secret := &corev1.Secret{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
			if err == nil {
				Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &QualysPlatformConfigReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalDefault))

			By("Checking the status condition")
			updatedResource := &qualysv1.QualysPlatformConfig{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedResource)).To(Succeed())
			Expect(updatedResource.Status.Conditions).NotTo(BeEmpty())

			credCond := qualysv1.GetCondition(updatedResource.Status.Conditions, qualysv1.ConditionTypeCredentialsReady)
			Expect(credCond).NotTo(BeNil())
			Expect(credCond.Status).To(Equal(metav1.ConditionTrue))
		})

		It("should fail when credentials secret is missing", func() {
			By("Creating a platform config with non-existent secret")
			badResource := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-platform-config",
				},
				Spec: qualysv1.QualysPlatformConfigSpec{
					Platform: qualysv1.PlatformSettings{
						ServerUri: "https://cmsqagpublic.qg2.apps.qualys.com/ContainerSensor",
					},
					Credentials: qualysv1.CredentialsConfig{
						SourceType: qualysv1.CredentialSourceSecret,
						SecretRef: &qualysv1.SecretReference{
							Name:      "nonexistent-secret",
							Namespace: namespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, badResource)).To(Succeed())

			controllerReconciler := &QualysPlatformConfigReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "bad-platform-config"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalError))

			Expect(k8sClient.Delete(ctx, badResource)).To(Succeed())
		})
	})
})
