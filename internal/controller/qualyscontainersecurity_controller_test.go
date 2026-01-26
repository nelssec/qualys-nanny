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

var _ = Describe("QualysContainerSecurity Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName       = "test-container-sensor"
			platformConfigName = "test-sensor-platform"
			secretName         = "test-sensor-credentials"
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
			platformConfig := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: platformConfigName,
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
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, &qualysv1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, platformConfig)).To(Succeed())
			}

			By("setting credentials ready condition on platform config")
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionTrue,
					Reason:             "CredentialsReady",
					Message:            "Credentials are available",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			By("creating the custom resource for the Kind QualysContainerSecurity")
			resource := &qualysv1.QualysContainerSecurity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: qualysv1.QualysContainerSecuritySpec{
					PlatformConfigRef: qualysv1.PlatformConfigReference{
						Name: platformConfigName,
					},
					ContainerSensor: &qualysv1.ContainerSensorConfig{
						Enabled: true,
						Image: &qualysv1.ImageSpec{
							Repository: "qualys/qcs-sensor",
							Tag:        "latest",
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1.QualysContainerSecurity{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the QualysContainerSecurity")
			resource := &qualysv1.QualysContainerSecurity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the QualysPlatformConfig")
			platformConfig := &qualysv1.QualysPlatformConfig{}
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
			controllerReconciler := &QualysContainerSecurityReconciler{
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
			platformConfig := &qualysv1.QualysPlatformConfig{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, platformConfig)).To(Succeed())
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionFalse,
					Reason:             "CredentialsNotReady",
					Message:            "Waiting for credentials",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			controllerReconciler := &QualysContainerSecurityReconciler{
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

		It("should handle when resource is not found", func() {
			By("Reconciling a non-existent resource")
			controllerReconciler := &QualysContainerSecurityReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "nonexistent",
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})

		It("should handle missing platform config", func() {
			By("Creating resource with non-existent platform config")
			resource := &qualysv1.QualysContainerSecurity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-missing-platform",
					Namespace: namespace,
				},
				Spec: qualysv1.QualysContainerSecuritySpec{
					PlatformConfigRef: qualysv1.PlatformConfigReference{
						Name: "nonexistent-platform",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &QualysContainerSecurityReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-missing-platform",
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueIntervalError))

			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})

	Context("When reconciling with cluster sensor enabled", func() {
		const (
			resourceName       = "test-cluster-sensor"
			platformConfigName = "test-cluster-platform"
			secretName         = "test-cluster-credentials"
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
			platformConfig := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: platformConfigName,
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
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, &qualysv1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, platformConfig)).To(Succeed())
			}

			By("setting credentials ready condition on platform config")
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionTrue,
					Reason:             "CredentialsReady",
					Message:            "Credentials are available",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			By("creating the custom resource with cluster sensor enabled")
			resource := &qualysv1.QualysContainerSecurity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: qualysv1.QualysContainerSecuritySpec{
					PlatformConfigRef: qualysv1.PlatformConfigReference{
						Name: platformConfigName,
					},
					ContainerSensor: &qualysv1.ContainerSensorConfig{
						Enabled: false,
					},
					ClusterSensor: &qualysv1.ClusterSensorConfig{
						Enabled: true,
						Image: &qualysv1.ImageSpec{
							Repository: "qualys/cluster-sensor",
							Tag:        "1.4.0-0",
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1.QualysContainerSecurity{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the QualysContainerSecurity")
			resource := &qualysv1.QualysContainerSecurity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the QualysPlatformConfig")
			platformConfig := &qualysv1.QualysPlatformConfig{}
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

		It("should reconcile cluster sensor resources", func() {
			By("Reconciling the resource with cluster sensor")
			controllerReconciler := &QualysContainerSecurityReconciler{
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
	})

	Context("When reconciling with unprivileged mode", func() {
		const (
			resourceName       = "test-unprivileged"
			platformConfigName = "test-unpriv-platform"
			secretName         = "test-unpriv-credentials"
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
			platformConfig := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: platformConfigName,
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
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, &qualysv1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, platformConfig)).To(Succeed())
			}

			By("setting credentials ready condition on platform config")
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionTrue,
					Reason:             "CredentialsReady",
					Message:            "Credentials are available",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			gid := int64(993)
			By("creating the custom resource with unprivileged mode")
			resource := &qualysv1.QualysContainerSecurity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: qualysv1.QualysContainerSecuritySpec{
					PlatformConfigRef: qualysv1.PlatformConfigReference{
						Name: platformConfigName,
					},
					ContainerSensor: &qualysv1.ContainerSensorConfig{
						Enabled:       true,
						PrivilegeMode: qualysv1.PrivilegeModeUnprivileged,
						Image: &qualysv1.ImageSpec{
							Repository: "qualys/qcs-sensor",
							Tag:        "latest",
						},
						Security: &qualysv1.SensorSecurityConfig{
							RuntimeSocketGroup: &gid,
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1.QualysContainerSecurity{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the QualysContainerSecurity")
			resource := &qualysv1.QualysContainerSecurity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the QualysPlatformConfig")
			platformConfig := &qualysv1.QualysPlatformConfig{}
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

		It("should reconcile with unprivileged settings", func() {
			By("Reconciling the unprivileged resource")
			controllerReconciler := &QualysContainerSecurityReconciler{
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
	})

	Context("When reconciling with runtime sensor enabled", func() {
		const (
			resourceName       = "test-runtime-sensor"
			platformConfigName = "test-runtime-platform"
			secretName         = "test-runtime-credentials"
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
			platformConfig := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: platformConfigName,
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
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, &qualysv1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, platformConfig)).To(Succeed())
			}

			By("setting credentials ready condition on platform config")
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionTrue,
					Reason:             "CredentialsReady",
					Message:            "Credentials are available",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			By("creating the custom resource with runtime sensor enabled")
			resource := &qualysv1.QualysContainerSecurity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: qualysv1.QualysContainerSecuritySpec{
					PlatformConfigRef: qualysv1.PlatformConfigReference{
						Name: platformConfigName,
					},
					ContainerSensor: &qualysv1.ContainerSensorConfig{
						Enabled: false,
					},
					RuntimeSensor: &qualysv1.RuntimeSensorConfig{
						Enabled: true,
						Image: &qualysv1.ImageSpec{
							Repository: "qualys/runtime-sensor",
							Tag:        "1.4.0-0",
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1.QualysContainerSecurity{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the QualysContainerSecurity")
			resource := &qualysv1.QualysContainerSecurity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the QualysPlatformConfig")
			platformConfig := &qualysv1.QualysPlatformConfig{}
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

		It("should reconcile runtime sensor resources", func() {
			By("Reconciling the resource with runtime sensor")
			controllerReconciler := &QualysContainerSecurityReconciler{
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
	})

	Context("When reconciling with admission controller enabled", func() {
		const (
			resourceName       = "test-admission-ctrl"
			platformConfigName = "test-admission-platform"
			secretName         = "test-admission-credentials"
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
			platformConfig := &qualysv1.QualysPlatformConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: platformConfigName,
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
			err = k8sClient.Get(ctx, types.NamespacedName{Name: platformConfigName}, &qualysv1.QualysPlatformConfig{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, platformConfig)).To(Succeed())
			}

			By("setting credentials ready condition on platform config")
			platformConfig.Status.Conditions = []metav1.Condition{
				{
					Type:               qualysv1.ConditionTypeCredentialsReady,
					Status:             metav1.ConditionTrue,
					Reason:             "CredentialsReady",
					Message:            "Credentials are available",
					LastTransitionTime: metav1.Now(),
				},
			}
			Expect(k8sClient.Status().Update(ctx, platformConfig)).To(Succeed())

			replicas := int32(2)
			By("creating the custom resource with admission controller enabled")
			resource := &qualysv1.QualysContainerSecurity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: qualysv1.QualysContainerSecuritySpec{
					PlatformConfigRef: qualysv1.PlatformConfigReference{
						Name: platformConfigName,
					},
					ContainerSensor: &qualysv1.ContainerSensorConfig{
						Enabled: false,
					},
					AdmissionController: &qualysv1.AdmissionControllerConfig{
						Enabled:  true,
						Replicas: &replicas,
						Image: &qualysv1.ImageSpec{
							Repository: "qualys/admission-controller",
							Tag:        "1.1.2-0",
						},
					},
				},
			}
			err = k8sClient.Get(ctx, typeNamespacedName, &qualysv1.QualysContainerSecurity{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			By("Cleanup the QualysContainerSecurity")
			resource := &qualysv1.QualysContainerSecurity{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("Cleanup the QualysPlatformConfig")
			platformConfig := &qualysv1.QualysPlatformConfig{}
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

		It("should reconcile admission controller resources", func() {
			By("Reconciling the resource with admission controller")
			controllerReconciler := &QualysContainerSecurityReconciler{
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
	})
})
