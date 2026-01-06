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

	qualysv1alpha1 "github.com/nelssec/qualys-nanny/api/v1alpha1"
)

var _ = Describe("QualysCloudAgent Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName       = "test-cloud-agent"
			platformConfigName = "test-platform"
			secretName         = "test-agent-credentials"
			namespace          = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: namespace,
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

			By("creating the QualysPlatformConfig")
			platformConfig := &qualysv1alpha1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: platformConfigName,
				},
				Spec: qualysv1alpha1.QualysPlatformConfigSpec{
					Platform: qualysv1alpha1.PlatformSettings{
						ServerUri: "https://qagpublic.qg2.apps.qualys.com/CloudAgent/",
					},
					Credentials: qualysv1alpha1.CredentialsConfig{
						SourceType: qualysv1alpha1.CredentialSourceSecret,
						SecretRef: &qualysv1alpha1.SecretReference{
							Name:      secretName,
							Namespace: namespace,
						},
					},
				},
			}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, &qualysv1alpha1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, platformConfig)).To(Succeed())
			}

			By("setting credentials ready condition on platform config")
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1alpha1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionTrue,
					Reason:             "CredentialsReady",
					Message:            "Credentials are available",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			By("creating the custom resource for the Kind QualysCloudAgent")
			resource := &qualysv1alpha1.QualysCloudAgent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: qualysv1alpha1.QualysCloudAgentSpec{
					PlatformConfigRef: qualysv1alpha1.PlatformConfigReference{
						Name: platformConfigName,
					},
					Image: &qualysv1alpha1.ImageSpec{
						Repository: "nelssec/qualys-agent-bootstrapper",
						Tag:        "v2.1.0",
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1alpha1.QualysCloudAgent{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the QualysCloudAgent")
			resource := &qualysv1alpha1.QualysCloudAgent{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the QualysPlatformConfig")
			platformConfig := &qualysv1alpha1.QualysPlatformConfig{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, platformConfig)
			if err == nil {
				Expect(k8sClient.Delete(ctx, platformConfig)).To(Succeed())
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
			controllerReconciler := &QualysCloudAgentReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalDefault))
		})

		It("should wait when platform config credentials are not ready", func() {
			By("Updating platform config to have credentials not ready")
			platformConfig := &qualysv1alpha1.QualysPlatformConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, platformConfig)).To(Succeed())
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1alpha1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionFalse,
					Reason:             "CredentialsNotReady",
					Message:            "Waiting for credentials",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			controllerReconciler := &QualysCloudAgentReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalError))
		})
	})
})
