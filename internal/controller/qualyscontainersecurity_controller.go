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
	"fmt"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
	"github.com/nelssec/qualys-nanny/internal/credentials"
	"github.com/nelssec/qualys-nanny/internal/platform"
	"github.com/nelssec/qualys-nanny/internal/resources"
)

type QualysContainerSecurityReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	DynamicClient dynamic.Interface
}

// +kubebuilder:rbac:groups=qualys.io,resources=qualyscontainersecurities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=qualys.io,resources=qualyscontainersecurities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=qualys.io,resources=qualyscontainersecurities/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts;configmaps;secrets;nodes;services;endpoints;namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes/status;pods;pods/status;pods/exec;replicationcontrollers/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets;daemonsets/status;deployments;deployments/status;replicasets;replicasets/status;statefulsets;statefulsets/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs;jobs/status;cronjobs;cronjobs/status,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses;networkpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=podsecuritypolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups="*",resources="*",verbs=get;list

func (r *QualysContainerSecurityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sensor := &qualysv1.QualysContainerSecurity{}
	err := r.Get(ctx, req.NamespacedName, sensor)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("QualysContainerSecurity resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get QualysContainerSecurity")
		return ctrl.Result{}, err
	}

	if sensor.Status.Conditions == nil {
		sensor.Status.Conditions = []metav1.Condition{}
	}

	platformConfig := &qualysv1.QualysPlatformConfig{}
	err = r.Get(ctx, types.NamespacedName{Name: sensor.Spec.PlatformConfigRef.Name}, platformConfig)
	if err != nil {
		log.Error(err, "Failed to get QualysPlatformConfig", "name", sensor.Spec.PlatformConfigRef.Name)
		r.setCondition(sensor, qualysv1.ConditionTypeAvailable, metav1.ConditionFalse, "PlatformConfigNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, sensor); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		r.Recorder.Event(sensor, corev1.EventTypeWarning, "PlatformConfigNotFound", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	credCond := qualysv1.GetCondition(platformConfig.Status.Conditions, qualysv1.ConditionTypeCredentialsReady)
	if credCond == nil || credCond.Status != metav1.ConditionTrue {
		log.Info("Credentials not ready, waiting")
		r.setCondition(sensor, qualysv1.ConditionTypeAvailable, metav1.ConditionFalse, "CredentialsNotReady", "Waiting for QualysPlatformConfig credentials")
		if updateErr := r.Status().Update(ctx, sensor); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	credResolver := credentials.NewResolver(r.Client)
	secretRef, err := credResolver.GetSecretRef(platformConfig)
	if err != nil {
		log.Error(err, "Failed to get secret reference")
		r.setCondition(sensor, qualysv1.ConditionTypeAvailable, metav1.ConditionFalse, "SecretRefError", err.Error())
		if updateErr := r.Status().Update(ctx, sensor); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	detectedRuntime := r.detectContainerRuntime(ctx, sensor)
	sensor.Status.DetectedRuntime = string(detectedRuntime)
	r.setCondition(sensor, qualysv1.ConditionTypeProgressing, metav1.ConditionTrue, "Reconciling", "Creating/updating resources")

	containerSensorCfg := sensor.Spec.GetContainerSensor()
	if containerSensorCfg.Enabled {
		if err := r.reconcileContainerSensor(ctx, sensor, platformConfig, secretRef.Name, detectedRuntime); err != nil {
			log.Error(err, "Failed to reconcile Container Sensor")
			return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
		}
	} else {
		if err := r.cleanupContainerSensor(ctx, sensor); err != nil {
			log.Error(err, "Failed to cleanup Container Sensor")
		}
	}

	clusterSensorCfg := sensor.Spec.GetClusterSensor()
	if clusterSensorCfg.Enabled {
		if err := r.reconcileClusterSensor(ctx, sensor, platformConfig, secretRef.Name); err != nil {
			log.Error(err, "Failed to reconcile Cluster Sensor")
			return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
		}
	} else {
		if err := r.cleanupClusterSensor(ctx, sensor); err != nil {
			log.Error(err, "Failed to cleanup Cluster Sensor")
		}
	}

	admissionCfg := sensor.Spec.GetAdmissionController()
	if admissionCfg.Enabled {
		if err := r.reconcileAdmissionController(ctx, sensor, platformConfig, secretRef.Name); err != nil {
			log.Error(err, "Failed to reconcile Admission Controller")
			return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
		}
	} else {
		if err := r.cleanupAdmissionController(ctx, sensor); err != nil {
			log.Error(err, "Failed to cleanup Admission Controller")
		}
	}

	runtimeSensorCfg := sensor.Spec.GetRuntimeSensor()
	if runtimeSensorCfg.Enabled {
		if err := r.reconcileRuntimeSensor(ctx, sensor, platformConfig, secretRef.Name, detectedRuntime); err != nil {
			log.Error(err, "Failed to reconcile Runtime Sensor")
			return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
		}
	} else {
		if err := r.cleanupRuntimeSensor(ctx, sensor); err != nil {
			log.Error(err, "Failed to cleanup Runtime Sensor")
		}
	}

	if err := r.updateComponentStatuses(ctx, sensor); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: RequeueIntervalDefault}, nil
}

func (r *QualysContainerSecurityReconciler) reconcileContainerSensor(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName string, rt platform.ContainerRuntime) error {
	log := logf.FromContext(ctx)
	baseName := sensor.Name + "-container"
	serviceAccountName := baseName + "-sa"
	configMapName := baseName + "-config"
	clusterRoleName := baseName + "-role"
	clusterRoleBindingName := baseName + "-rolebinding"
	sccName := baseName + "-scc"

	containerSensorCfg := sensor.Spec.GetContainerSensor()
	r.validatePrivilegeModeFeatures(sensor, &containerSensorCfg)

	if err := r.reconcileServiceAccount(ctx, sensor, serviceAccountName, "container-sensor"); err != nil {
		return err
	}
	if err := r.reconcileContainerSensorClusterRole(ctx, clusterRoleName); err != nil {
		return err
	}
	if err := r.reconcileClusterRoleBinding(ctx, sensor, clusterRoleBindingName, serviceAccountName, clusterRoleName); err != nil {
		return err
	}
	if platform.IsOpenShift(ctx) {
		if err := r.reconcileContainerSensorSCC(ctx, sensor, sccName, serviceAccountName); err != nil {
			log.Error(err, "Failed to reconcile Container Sensor SCC", "scc", sccName)
			r.Recorder.Event(sensor, corev1.EventTypeWarning, "SCCReconcileFailed", err.Error())
		}
	}

	if err := r.reconcileContainerSensorConfigMap(ctx, sensor, configMapName, platformConfig, containerSensorCfg); err != nil {
		return err
	}
	if err := r.reconcileContainerSensorDaemonSet(ctx, sensor, platformConfig, secretName, serviceAccountName, rt); err != nil {
		return err
	}

	return nil
}

func (r *QualysContainerSecurityReconciler) reconcileClusterSensor(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName string) error {
	log := logf.FromContext(ctx)
	baseName := sensor.Name + "-cluster"
	serviceAccountName := baseName + "-sa"
	clusterRoleName := baseName + "-role"
	clusterRoleBindingName := baseName + "-rolebinding"
	sccName := baseName + "-scc"

	if err := r.reconcileServiceAccount(ctx, sensor, serviceAccountName, "cluster-sensor"); err != nil {
		return err
	}
	if err := r.reconcileClusterSensorClusterRole(ctx, clusterRoleName); err != nil {
		return err
	}
	if err := r.reconcileClusterRoleBinding(ctx, sensor, clusterRoleBindingName, serviceAccountName, clusterRoleName); err != nil {
		return err
	}
	if platform.IsOpenShift(ctx) {
		if err := r.reconcileClusterSensorSCC(ctx, sensor, sccName, serviceAccountName); err != nil {
			log.Error(err, "Failed to reconcile Cluster Sensor SCC", "scc", sccName)
		}
	}

	if err := r.reconcileClusterSensorDeployment(ctx, sensor, platformConfig, secretName, serviceAccountName); err != nil {
		log.Error(err, "Failed to reconcile Cluster Sensor Deployment")
		return err
	}

	return nil
}

func (r *QualysContainerSecurityReconciler) reconcileAdmissionController(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName string) error {
	log := logf.FromContext(ctx)
	baseName := sensor.Name + "-admission"
	serviceAccountName := baseName + "-sa"
	clusterRoleName := baseName + "-role"
	clusterRoleBindingName := baseName + "-rolebinding"
	serviceName := baseName + "-svc"
	webhookName := baseName + "-webhook"

	if err := r.reconcileServiceAccount(ctx, sensor, serviceAccountName, "admission-controller"); err != nil {
		return err
	}
	if err := r.reconcileAdmissionClusterRole(ctx, clusterRoleName); err != nil {
		return err
	}
	if err := r.reconcileClusterRoleBinding(ctx, sensor, clusterRoleBindingName, serviceAccountName, clusterRoleName); err != nil {
		return err
	}

	if err := r.reconcileAdmissionService(ctx, sensor, serviceName); err != nil {
		log.Error(err, "Failed to reconcile Admission Controller Service")
		return err
	}
	if err := r.reconcileAdmissionDeployment(ctx, sensor, platformConfig, secretName, serviceAccountName); err != nil {
		log.Error(err, "Failed to reconcile Admission Controller Deployment")
		return err
	}
	if err := r.reconcileValidatingWebhook(ctx, sensor, webhookName, serviceName); err != nil {
		log.Error(err, "Failed to reconcile ValidatingWebhookConfiguration")
		return err
	}

	return nil
}

func (r *QualysContainerSecurityReconciler) reconcileRuntimeSensor(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName string, rt platform.ContainerRuntime) error {
	log := logf.FromContext(ctx)
	baseName := sensor.Name + "-runtime"
	serviceAccountName := baseName + "-sa"
	clusterRoleName := baseName + "-role"
	clusterRoleBindingName := baseName + "-rolebinding"
	sccName := baseName + "-scc"

	r.Recorder.Event(sensor, corev1.EventTypeWarning, "PrivilegedModeRequired",
		"Runtime Sensor (eBPF) requires privileged mode for kernel tracing. This is the only sensor component that requires privileged: true.")

	if err := r.reconcileServiceAccount(ctx, sensor, serviceAccountName, "runtime-sensor"); err != nil {
		return err
	}
	if err := r.reconcileRuntimeSensorClusterRole(ctx, clusterRoleName); err != nil {
		return err
	}
	if err := r.reconcileClusterRoleBinding(ctx, sensor, clusterRoleBindingName, serviceAccountName, clusterRoleName); err != nil {
		return err
	}
	if platform.IsOpenShift(ctx) {
		if err := r.reconcileRuntimeSensorSCC(ctx, sensor, sccName, serviceAccountName); err != nil {
			log.Error(err, "Failed to reconcile Runtime Sensor SCC", "scc", sccName)
		}
	}

	if err := r.reconcileRuntimeSensorDaemonSet(ctx, sensor, platformConfig, secretName, serviceAccountName, rt); err != nil {
		log.Error(err, "Failed to reconcile Runtime Sensor DaemonSet")
		return err
	}

	return nil
}

func (r *QualysContainerSecurityReconciler) detectContainerRuntime(_ context.Context, sensor *qualysv1.QualysContainerSecurity) platform.ContainerRuntime {
	runtimeConfig := sensor.Spec.GetContainerRuntime()
	switch runtimeConfig.Type {
	case qualysv1.ContainerRuntimeContainerd:
		return platform.RuntimeContainerd
	case qualysv1.ContainerRuntimeDocker:
		return platform.RuntimeDocker
	default:
		return platform.RuntimeCRIO
	}
}

func (r *QualysContainerSecurityReconciler) reconcileServiceAccount(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name, component string) error {
	sa := resources.BuildServiceAccount(name, sensor.Namespace, component)

	if err := controllerutil.SetControllerReference(sensor, sa, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, sa)
		}
		return err
	}

	existing.Labels = sa.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileContainerSensorClusterRole(ctx context.Context, name string) error {
	role := resources.BuildContainerSensorClusterRole(name)

	existing := &rbacv1.ClusterRole{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, role)
		}
		return err
	}

	existing.Rules = role.Rules
	existing.Labels = role.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileClusterSensorClusterRole(ctx context.Context, name string) error {
	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"pods", "namespaces", "nodes", "services", "serviceaccounts"}, Verbs: []string{"watch"}},
		{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"roles", "rolebindings", "clusterroles", "clusterrolebindings"}, Verbs: []string{"watch"}},
		{APIGroups: []string{"discovery.k8s.io"}, Resources: []string{"endpointslices"}, Verbs: []string{"watch"}},
		{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"watch"}},
		{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get", "list"}},
	}

	if platform.IsOpenShift(ctx) {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{"security.openshift.io"},
			Resources: []string{"securitycontextconstraints"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		})
	}

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "cluster-sensor",
			},
		},
		Rules: rules,
	}

	existing := &rbacv1.ClusterRole{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, role)
		}
		return err
	}

	existing.Rules = role.Rules
	existing.Labels = role.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileAdmissionClusterRole(ctx context.Context, name string) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "admission-controller",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"pods", "namespaces"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"apps"}, Resources: []string{"deployments", "daemonsets", "replicasets", "statefulsets"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs", "cronjobs"}, Verbs: []string{"get", "list", "watch"}},
		},
	}

	existing := &rbacv1.ClusterRole{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, role)
		}
		return err
	}

	existing.Rules = role.Rules
	existing.Labels = role.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileRuntimeSensorClusterRole(ctx context.Context, name string) error {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "runtime-sensor",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"nodes", "pods"}, Verbs: []string{"get", "list", "watch"}},
		},
	}

	existing := &rbacv1.ClusterRole{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, role)
		}
		return err
	}

	existing.Rules = role.Rules
	existing.Labels = role.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileClusterRoleBinding(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name, serviceAccountName, clusterRoleName string) error {
	binding := resources.BuildContainerSensorClusterRoleBinding(name, sensor.Namespace, serviceAccountName, clusterRoleName)

	existing := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, binding)
		}
		return err
	}

	existing.RoleRef = binding.RoleRef
	existing.Subjects = binding.Subjects
	existing.Labels = binding.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileContainerSensorSCC(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name, serviceAccountName string) error {
	if r.DynamicClient == nil {
		return fmt.Errorf("dynamic client not configured")
	}

	cfg := sensor.Spec.GetContainerSensor()
	scc := resources.BuildContainerSensorSCCWithPrivilegeMode(name, sensor.Namespace, serviceAccountName, cfg.PrivilegeMode)

	existing, err := r.DynamicClient.Resource(platform.SCCGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = r.DynamicClient.Resource(platform.SCCGVR).Create(ctx, scc, metav1.CreateOptions{})
			return err
		}
		return err
	}

	scc.SetResourceVersion(existing.GetResourceVersion())
	_, err = r.DynamicClient.Resource(platform.SCCGVR).Update(ctx, scc, metav1.UpdateOptions{})
	return err
}

func (r *QualysContainerSecurityReconciler) reconcileRuntimeSensorSCC(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name, serviceAccountName string) error {
	if r.DynamicClient == nil {
		return fmt.Errorf("dynamic client not configured")
	}

	scc := resources.BuildRuntimeSensorSCC(name, sensor.Namespace, serviceAccountName)

	existing, err := r.DynamicClient.Resource(platform.SCCGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = r.DynamicClient.Resource(platform.SCCGVR).Create(ctx, scc, metav1.CreateOptions{})
			return err
		}
		return err
	}

	scc.SetResourceVersion(existing.GetResourceVersion())
	_, err = r.DynamicClient.Resource(platform.SCCGVR).Update(ctx, scc, metav1.UpdateOptions{})
	return err
}

func (r *QualysContainerSecurityReconciler) reconcileClusterSensorSCC(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name, serviceAccountName string) error {
	if r.DynamicClient == nil {
		return fmt.Errorf("dynamic client not configured")
	}

	scc := resources.BuildClusterSensorSCC(name, sensor.Namespace, serviceAccountName)

	existing, err := r.DynamicClient.Resource(platform.SCCGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = r.DynamicClient.Resource(platform.SCCGVR).Create(ctx, scc, metav1.CreateOptions{})
			return err
		}
		return err
	}

	scc.SetResourceVersion(existing.GetResourceVersion())
	_, err = r.DynamicClient.Resource(platform.SCCGVR).Update(ctx, scc, metav1.UpdateOptions{})
	return err
}

func (r *QualysContainerSecurityReconciler) reconcileContainerSensorConfigMap(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name string, platformConfig *qualysv1.QualysPlatformConfig, sensorConfig qualysv1.ContainerSensorConfig) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sensor.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "container-sensor",
			},
		},
		Data: map[string]string{
			"QUALYS_GATEWAY_URL": platformConfig.Spec.Platform.ServerUri,
			"SENSOR_MODE":        string(sensorConfig.Mode),
		},
	}

	if err := controllerutil.SetControllerReference(sensor, cm, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, cm)
		}
		return err
	}

	existing.Data = cm.Data
	existing.Labels = cm.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileContainerSensorDaemonSet(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string, rt platform.ContainerRuntime) error {
	ds := r.buildContainerSensorDaemonSet(sensor, platformConfig, secretName, serviceAccountName, rt)

	if err := controllerutil.SetControllerReference(sensor, ds, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(sensor, corev1.EventTypeNormal, "ContainerSensorCreated", "Created Container Sensor DaemonSet")
			return r.Create(ctx, ds)
		}
		return err
	}

	existing.Spec = ds.Spec
	existing.Labels = ds.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileClusterSensorDeployment(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string) error {
	deploy := r.buildClusterSensorDeployment(sensor, platformConfig, secretName, serviceAccountName)

	if err := controllerutil.SetControllerReference(sensor, deploy, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(sensor, corev1.EventTypeNormal, "ClusterSensorCreated", "Created Cluster Sensor Deployment")
			return r.Create(ctx, deploy)
		}
		return err
	}

	existing.Spec = deploy.Spec
	existing.Labels = deploy.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileAdmissionService(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sensor.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "qualys-admission-controller",
				"app.kubernetes.io/instance":   sensor.Name,
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "admission-controller",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name":      "qualys-admission-controller",
				"app.kubernetes.io/instance":  sensor.Name,
				"app.kubernetes.io/component": "admission-controller",
			},
			Ports: []corev1.ServicePort{
				{Name: "https", Port: 443, TargetPort: intstr.FromInt(8443), Protocol: corev1.ProtocolTCP},
			},
		},
	}

	if err := controllerutil.SetControllerReference(sensor, svc, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, svc)
		}
		return err
	}

	existing.Spec.Selector = svc.Spec.Selector
	existing.Spec.Ports = svc.Spec.Ports
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileAdmissionDeployment(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string) error {
	deploy := r.buildAdmissionDeployment(sensor, platformConfig, secretName, serviceAccountName)

	if err := controllerutil.SetControllerReference(sensor, deploy, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(sensor, corev1.EventTypeNormal, "AdmissionControllerCreated", "Created Admission Controller Deployment")
			return r.Create(ctx, deploy)
		}
		return err
	}

	existing.Spec = deploy.Spec
	existing.Labels = deploy.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileValidatingWebhook(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, name, serviceName string) error {
	admissionCfg := sensor.Spec.GetAdmissionController()
	failurePolicy := admissionv1.Ignore
	if admissionCfg.FailurePolicy == "Fail" {
		failurePolicy = admissionv1.Fail
	}
	sideEffects := admissionv1.SideEffectClassNone

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "qualys-nanny",
				"app.kubernetes.io/component":  "admission-controller",
			},
		},
		Webhooks: []admissionv1.ValidatingWebhook{
			{
				Name: "qualys-admission.qualys.io",
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Name:      serviceName,
						Namespace: sensor.Namespace,
						Path:      strPtr("/validate"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{
					{
						Operations: []admissionv1.OperationType{admissionv1.Create, admissionv1.Update},
						Rule: admissionv1.Rule{
							APIGroups:   []string{"", "apps", "batch"},
							APIVersions: []string{"v1", "v1beta1"},
							Resources:   []string{"pods", "deployments", "daemonsets", "statefulsets", "replicasets", "jobs", "cronjobs"},
						},
					},
				},
				FailurePolicy:           &failurePolicy,
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				NamespaceSelector:       admissionCfg.NamespaceSelector,
			},
		},
	}

	existing := &admissionv1.ValidatingWebhookConfiguration{}
	err := r.Get(ctx, types.NamespacedName{Name: name}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, webhook)
		}
		return err
	}

	existing.Webhooks = webhook.Webhooks
	existing.Labels = webhook.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) reconcileRuntimeSensorDaemonSet(ctx context.Context, sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string, rt platform.ContainerRuntime) error {
	ds := r.buildRuntimeSensorDaemonSet(sensor, platformConfig, secretName, serviceAccountName, rt)

	if err := controllerutil.SetControllerReference(sensor, ds, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(sensor, corev1.EventTypeNormal, "RuntimeSensorCreated", "Created Runtime Sensor DaemonSet")
			return r.Create(ctx, ds)
		}
		return err
	}

	existing.Spec = ds.Spec
	existing.Labels = ds.Labels
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) buildContainerSensorDaemonSet(sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string, rt platform.ContainerRuntime) *appsv1.DaemonSet {
	cfg := sensor.Spec.GetContainerSensor()
	scheduling := sensor.Spec.GetScheduling()

	labels := map[string]string{
		"app.kubernetes.io/name":       "qualys-container-sensor",
		"app.kubernetes.io/instance":   sensor.Name,
		"app.kubernetes.io/managed-by": "qualys-nanny",
		"app.kubernetes.io/component":  "container-sensor",
	}

	runtimeConfig := sensor.Spec.GetContainerRuntime()
	socketPath := getSocketPath(runtimeConfig, rt)
	runtimeName := getRuntimeName(rt)

	args := buildContainerSensorArgs(cfg, runtimeName)
	envVars := buildContainerSensorEnvVars(secretName, platformConfig.Spec.Platform.ServerUri)
	volumeMounts, volumes := buildContainerSensorVolumes(socketPath, rt, cfg.PrivilegeMode)

	var resourceReqs corev1.ResourceRequirements
	if cfg.Resources != nil {
		resourceReqs = *cfg.Resources
	}

	maxUnavailable := intstr.FromString("25%")
	if cfg.UpdateStrategy != nil && cfg.UpdateStrategy.RollingUpdate != nil {
		maxUnavailable = intstr.FromString(cfg.UpdateStrategy.RollingUpdate.MaxUnavailable)
	}

	securityContext := buildContainerSensorSecurityContext(&cfg)
	podSecurityContext := buildContainerSensorPodSecurityContext(&cfg, rt)
	hostNetwork := cfg.GetEffectiveHostNetwork()
	hostPID := cfg.GetEffectiveHostPID()

	dnsPolicy := corev1.DNSClusterFirst
	if hostNetwork {
		dnsPolicy = corev1.DNSClusterFirstWithHostNet
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sensor.Name + "-container",
			Namespace: sensor.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type:          appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: &maxUnavailable},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName:            serviceAccountName,
					HostNetwork:                   hostNetwork,
					HostPID:                       hostPID,
					PriorityClassName:             scheduling.PriorityClassName,
					NodeSelector:                  scheduling.NodeSelector,
					Tolerations:                   scheduling.Tolerations,
					Affinity:                      scheduling.Affinity,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     dnsPolicy,
					TerminationGracePeriodSeconds: int64Ptr(30),
					SecurityContext:               podSecurityContext,
					Containers: []corev1.Container{{
						Name:            "qualys-container-sensor",
						Image:           fmt.Sprintf("%s:%s", cfg.Image.Repository, cfg.Image.Tag),
						ImagePullPolicy: cfg.Image.PullPolicy,
						Args:            args,
						Env:             envVars,
						VolumeMounts:    volumeMounts,
						SecurityContext: securityContext,
						Resources:       resourceReqs,
					}},
					Volumes:          volumes,
					ImagePullSecrets: cfg.Image.PullSecrets,
				},
			},
		},
	}
}

func buildContainerSensorSecurityContext(cfg *qualysv1.ContainerSensorConfig) *corev1.SecurityContext {
	seccompProfile := &corev1.SeccompProfile{
		Type: cfg.GetEffectiveSeccompProfile(),
	}

	switch cfg.PrivilegeMode {
	case qualysv1.PrivilegeModeUnprivileged:
		return &corev1.SecurityContext{
			Privileged:               boolPtr(false),
			RunAsUser:                cfg.GetEffectiveRunAsUser(),
			RunAsGroup:               cfg.GetEffectiveRunAsGroup(),
			RunAsNonRoot:             boolPtr(true),
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(cfg.GetEffectiveReadOnlyRootFilesystem()),
			SeccompProfile:           seccompProfile,
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		}

	case qualysv1.PrivilegeModeMinimal:
		return &corev1.SecurityContext{
			Privileged:               boolPtr(false),
			RunAsUser:                int64Ptr(0),
			RunAsNonRoot:             boolPtr(false),
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(cfg.GetEffectiveReadOnlyRootFilesystem()),
			SeccompProfile:           seccompProfile,
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"SYS_PTRACE"},
			},
		}

	case qualysv1.PrivilegeModePrivileged:
		return &corev1.SecurityContext{
			Privileged:               boolPtr(true),
			RunAsUser:                int64Ptr(0),
			RunAsNonRoot:             boolPtr(false),
			AllowPrivilegeEscalation: boolPtr(true),
			ReadOnlyRootFilesystem:   boolPtr(false),
		}

	default:
		return &corev1.SecurityContext{
			Privileged:               boolPtr(false),
			RunAsUser:                int64Ptr(0),
			RunAsNonRoot:             boolPtr(false),
			AllowPrivilegeEscalation: boolPtr(true),
			ReadOnlyRootFilesystem:   boolPtr(false),
			SeccompProfile:           seccompProfile,
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add: []corev1.Capability{
					"SYS_ADMIN",
					"SYS_PTRACE",
					"SYS_CHROOT",
					"DAC_READ_SEARCH",
				},
			},
		}
	}
}

func buildContainerSensorPodSecurityContext(cfg *qualysv1.ContainerSensorConfig, rt platform.ContainerRuntime) *corev1.PodSecurityContext {
	psc := &corev1.PodSecurityContext{}

	if cfg.PrivilegeMode == qualysv1.PrivilegeModeUnprivileged {
		psc.RunAsNonRoot = boolPtr(true)
		psc.RunAsUser = cfg.GetEffectiveRunAsUser()
		psc.RunAsGroup = cfg.GetEffectiveRunAsGroup()

		socketGID := cfg.GetRuntimeSocketGroup()
		if socketGID == nil {
			defaultGID := getDefaultSocketGID(rt)
			socketGID = &defaultGID
		}
		psc.SupplementalGroups = []int64{*socketGID}
		psc.FSGroup = socketGID

		psc.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	return psc
}

func getDefaultSocketGID(_ platform.ContainerRuntime) int64 {
	return 0
}

func (r *QualysContainerSecurityReconciler) buildClusterSensorDeployment(sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string) *appsv1.Deployment {
	cfg := sensor.Spec.GetClusterSensor()

	labels := map[string]string{
		"app.kubernetes.io/name":       "qualys-cluster-sensor",
		"app.kubernetes.io/instance":   sensor.Name,
		"app.kubernetes.io/managed-by": "qualys-nanny",
		"app.kubernetes.io/component":  "cluster-sensor",
	}

	var resourceReqs corev1.ResourceRequirements
	if cfg.Resources != nil {
		resourceReqs = *cfg.Resources
	} else {
		resourceReqs = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("750Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		}
	}

	args := r.buildClusterSensorArgs(cfg, platformConfig)

	isOpenShift := platform.IsOpenShift(context.Background())

	hostScannerEnabled := cfg.HostScanner != nil && cfg.HostScanner.Enabled
	hostScannerRunOnMaster := cfg.HostScanner != nil && cfg.HostScanner.RunOnMaster

	args = append(args, fmt.Sprintf("--openshift=%t", isOpenShift))
	args = append(args, fmt.Sprintf("--enable-k8s-compliance=%t", cfg.K8sCompliance))
	args = append(args, fmt.Sprintf("--host-scanner-enable=%t", hostScannerEnabled))
	args = append(args, fmt.Sprintf("--host-scanner-run-on-master=%t", hostScannerRunOnMaster))

	if hostScannerEnabled && cfg.HostScanner.Resources != nil {
		if cpu := cfg.HostScanner.Resources.Limits.Cpu(); cpu != nil {
			args = append(args, "--host-scanner-cpu-limit", cpu.String())
		}
		if mem := cfg.HostScanner.Resources.Limits.Memory(); mem != nil {
			args = append(args, "--host-scanner-memory-limit", mem.String())
		}
	} else if hostScannerEnabled {
		args = append(args, "--host-scanner-cpu-limit", "100m")
		args = append(args, "--host-scanner-memory-limit", "256Mi")
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sensor.Name + "-cluster",
			Namespace: sensor.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: cfg.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName:            serviceAccountName,
					TerminationGracePeriodSeconds: int64Ptr(120),
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: int64Ptr(555),
					},
					Containers: []corev1.Container{{
						Name:            "qualys-cluster-sensor",
						Image:           fmt.Sprintf("%s:%s", cfg.Image.Repository, cfg.Image.Tag),
						ImagePullPolicy: cfg.Image.PullPolicy,
						Args:            args,
						SecurityContext: &corev1.SecurityContext{
							RunAsUser:                int64Ptr(555),
							RunAsGroup:               int64Ptr(555),
							RunAsNonRoot:             boolPtr(true),
							AllowPrivilegeEscalation: boolPtr(false),
						},
						Env: []corev1.EnvVar{
							{Name: "CLUSTERSENSOR_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
							{Name: "CLUSTERSENSOR_POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
						},
						Resources: resourceReqs,
					}},
					ImagePullSecrets: cfg.Image.PullSecrets,
				},
			},
		},
	}
}

func (r *QualysContainerSecurityReconciler) buildClusterSensorArgs(cfg qualysv1.ClusterSensorConfig, platformConfig *qualysv1.QualysPlatformConfig) []string {
	credResolver := credentials.NewResolver(r.Client)
	creds, _ := credResolver.Resolve(context.Background(), platformConfig)

	gatewayUrl := platformConfig.Spec.Platform.GatewayUrl
	if gatewayUrl == "" {
		gatewayUrl = platformConfig.Spec.Platform.ServerUri
	}

	args := []string{
		"--customer-id", creds.CustomerID,
		"--activation-id", creds.ActivationID,
		"--gateway-url", gatewayUrl,
	}

	if cfg.Logging != nil {
		logLevel := "info"
		switch cfg.Logging.LogLevel {
		case 0, 1:
			logLevel = "error"
		case 2:
			logLevel = "warn"
		case 3:
			logLevel = "info"
		case 4, 5:
			logLevel = "debug"
		}
		args = append(args, "--log-level", logLevel)
	}

	args = append(args, "--cloud-provider", string(cfg.CloudProvider))

	switch cfg.CloudProvider {
	case qualysv1.CloudProviderAWS:
		if cfg.ClusterID != "" {
			args = append(args, "--cluster-id", cfg.ClusterID)
		}
	case qualysv1.CloudProviderAzure:
		if cfg.ClusterID != "" {
			args = append(args, "--cluster-id", cfg.ClusterID)
		}
		if cfg.ClusterRegion != "" {
			args = append(args, "--cluster-region", cfg.ClusterRegion)
		}
	case qualysv1.CloudProviderGCP:
		if cfg.ClusterID != "" {
			args = append(args, "--cluster-id", cfg.ClusterID)
		}
	case qualysv1.CloudProviderOCI:
		if cfg.ClusterID != "" {
			args = append(args, "--cluster-id", cfg.ClusterID)
		}
		if cfg.ClusterName != "" {
			args = append(args, "--cluster-name", cfg.ClusterName)
		}
	case qualysv1.CloudProviderSelfManagedK8S:
		if cfg.ClusterName != "" {
			args = append(args, "--cluster-name", cfg.ClusterName)
		}
	}

	args = append(args, cfg.ExtraArgs...)

	return args
}

func (r *QualysContainerSecurityReconciler) buildAdmissionDeployment(sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string) *appsv1.Deployment {
	cfg := sensor.Spec.GetAdmissionController()

	labels := map[string]string{
		"app.kubernetes.io/name":       "qualys-admission-controller",
		"app.kubernetes.io/instance":   sensor.Name,
		"app.kubernetes.io/managed-by": "qualys-nanny",
		"app.kubernetes.io/component":  "admission-controller",
	}

	var resourceReqs corev1.ResourceRequirements
	if cfg.Resources != nil {
		resourceReqs = *cfg.Resources
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sensor.Name + "-admission",
			Namespace: sensor.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: cfg.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{{
						Name:            "admission-controller",
						Image:           fmt.Sprintf("%s:%s", cfg.Image.Repository, cfg.Image.Tag),
						ImagePullPolicy: cfg.Image.PullPolicy,
						Ports:           []corev1.ContainerPort{{ContainerPort: 8443, Protocol: corev1.ProtocolTCP}},
						Env: []corev1.EnvVar{
							{Name: "ACTIVATIONID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "ACTIVATION_ID"}}},
							{Name: "CUSTOMERID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "CUSTOMER_ID"}}},
							{Name: "QUALYS_GATEWAY_URL", Value: platformConfig.Spec.Platform.ServerUri},
						},
						Resources: resourceReqs,
					}},
					ImagePullSecrets: cfg.Image.PullSecrets,
				},
			},
		},
	}
}

func (r *QualysContainerSecurityReconciler) buildRuntimeSensorDaemonSet(sensor *qualysv1.QualysContainerSecurity, platformConfig *qualysv1.QualysPlatformConfig, secretName, serviceAccountName string, rt platform.ContainerRuntime) *appsv1.DaemonSet {
	cfg := sensor.Spec.GetRuntimeSensor()
	scheduling := sensor.Spec.GetScheduling()
	clusterCfg := sensor.Spec.GetClusterSensor()

	labels := map[string]string{
		"app.kubernetes.io/name":       "qualys-runtime-sensor",
		"app.kubernetes.io/instance":   sensor.Name,
		"app.kubernetes.io/managed-by": "qualys-nanny",
		"app.kubernetes.io/component":  "runtime-sensor",
	}

	var resourceReqs corev1.ResourceRequirements
	if cfg.Resources != nil {
		resourceReqs = *cfg.Resources
	} else {
		resourceReqs = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("800m"),
				corev1.ResourceMemory: resource.MustParse("1024Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1024Mi"),
			},
		}
	}

	maxUnavailable := intstr.FromString("25%")
	if cfg.UpdateStrategy != nil && cfg.UpdateStrategy.RollingUpdate != nil {
		maxUnavailable = intstr.FromString(cfg.UpdateStrategy.RollingUpdate.MaxUnavailable)
	}

	args := r.buildRuntimeSensorArgs(cfg, clusterCfg, platformConfig)

	volumeMounts := []corev1.VolumeMount{
		{Name: "proc-root", MountPath: "/procRoot/", ReadOnly: true},
	}
	volumes := []corev1.Volume{
		{Name: "proc-root", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/proc", Type: hostPathTypePtr(corev1.HostPathDirectory)}}},
	}

	if rt == platform.RuntimeCRIO {
		runtimeConfig := sensor.Spec.GetContainerRuntime()
		socketPath := "/var/run/crio/crio.sock"
		if runtimeConfig.SocketPaths != nil && runtimeConfig.SocketPaths.CRIO != "" {
			socketPath = runtimeConfig.SocketPaths.CRIO
		}
		volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: "socket-volume", MountPath: socketPath, ReadOnly: true})
		volumes = append(volumes, corev1.Volume{Name: "socket-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: socketPath, Type: hostPathTypePtr(corev1.HostPathSocket)}}})
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sensor.Name + "-runtime",
			Namespace: sensor.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type:          appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{MaxUnavailable: &maxUnavailable},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName:            serviceAccountName,
					HostNetwork:                   true,
					PriorityClassName:             scheduling.PriorityClassName,
					NodeSelector:                  scheduling.NodeSelector,
					Tolerations:                   scheduling.Tolerations,
					Affinity:                      scheduling.Affinity,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirstWithHostNet,
					TerminationGracePeriodSeconds: int64Ptr(30),
					Containers: []corev1.Container{{
						Name:            "qualys-runtime-sensor",
						Image:           fmt.Sprintf("%s:%s", cfg.Image.Repository, cfg.Image.Tag),
						ImagePullPolicy: cfg.Image.PullPolicy,
						Args:            args,
						Env: []corev1.EnvVar{
							{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}}},
							{Name: "QUALYS_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
							{Name: "QUALYS_POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               boolPtr(true),
							RunAsUser:                int64Ptr(0),
							RunAsNonRoot:             boolPtr(false),
							AllowPrivilegeEscalation: boolPtr(true),
						},
						Resources:    resourceReqs,
						VolumeMounts: volumeMounts,
					}},
					Volumes:          volumes,
					ImagePullSecrets: cfg.Image.PullSecrets,
				},
			},
		},
	}
}

func (r *QualysContainerSecurityReconciler) buildRuntimeSensorArgs(cfg qualysv1.RuntimeSensorConfig, clusterCfg qualysv1.ClusterSensorConfig, platformConfig *qualysv1.QualysPlatformConfig) []string {
	credResolver := credentials.NewResolver(r.Client)
	creds, _ := credResolver.Resolve(context.Background(), platformConfig)

	gatewayUrl := platformConfig.Spec.Platform.GatewayUrl
	if gatewayUrl == "" {
		gatewayUrl = platformConfig.Spec.Platform.ServerUri
	}

	args := []string{
		"--customer-id", creds.CustomerID,
		"--activation-id", creds.ActivationID,
		"--gateway-url", gatewayUrl,
	}

	if cfg.Logging != nil {
		logLevel := "info"
		switch cfg.Logging.LogLevel {
		case 0, 1:
			logLevel = "error"
		case 2:
			logLevel = "warn"
		case 3:
			logLevel = "info"
		case 4, 5:
			logLevel = "debug"
		}
		args = append(args, "--log-level", logLevel)
	}

	args = append(args, "--cloud-provider", string(clusterCfg.CloudProvider))

	switch clusterCfg.CloudProvider {
	case qualysv1.CloudProviderAWS:
		if clusterCfg.ClusterID != "" {
			args = append(args, "--cluster-id", clusterCfg.ClusterID)
		}
	case qualysv1.CloudProviderAzure:
		if clusterCfg.ClusterID != "" {
			args = append(args, "--cluster-id", clusterCfg.ClusterID)
		}
		if clusterCfg.ClusterRegion != "" {
			args = append(args, "--cluster-region", clusterCfg.ClusterRegion)
		}
	case qualysv1.CloudProviderGCP:
		if clusterCfg.ClusterID != "" {
			args = append(args, "--cluster-id", clusterCfg.ClusterID)
		}
	case qualysv1.CloudProviderOCI:
		if clusterCfg.ClusterID != "" {
			args = append(args, "--cluster-id", clusterCfg.ClusterID)
		}
		if clusterCfg.ClusterName != "" {
			args = append(args, "--cluster-name", clusterCfg.ClusterName)
		}
	case qualysv1.CloudProviderSelfManagedK8S:
		if clusterCfg.ClusterName != "" {
			args = append(args, "--cluster-name", clusterCfg.ClusterName)
		}
	}

	args = append(args, cfg.ExtraArgs...)

	return args
}

func hostPathTypePtr(t corev1.HostPathType) *corev1.HostPathType { return &t }

func (r *QualysContainerSecurityReconciler) cleanupContainerSensor(ctx context.Context, sensor *qualysv1.QualysContainerSecurity) error {
	dsName := sensor.Name + "-container"
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: dsName, Namespace: sensor.Namespace}, ds); err == nil {
		if err := r.Delete(ctx, ds); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *QualysContainerSecurityReconciler) cleanupClusterSensor(ctx context.Context, sensor *qualysv1.QualysContainerSecurity) error {
	deployName := sensor.Name + "-cluster"
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployName, Namespace: sensor.Namespace}, deploy); err == nil {
		if err := r.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *QualysContainerSecurityReconciler) cleanupAdmissionController(ctx context.Context, sensor *qualysv1.QualysContainerSecurity) error {
	deployName := sensor.Name + "-admission"
	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployName, Namespace: sensor.Namespace}, deploy); err == nil {
		if err := r.Delete(ctx, deploy); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	webhookName := sensor.Name + "-admission-webhook"
	webhook := &admissionv1.ValidatingWebhookConfiguration{}
	if err := r.Get(ctx, types.NamespacedName{Name: webhookName}, webhook); err == nil {
		if err := r.Delete(ctx, webhook); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *QualysContainerSecurityReconciler) cleanupRuntimeSensor(ctx context.Context, sensor *qualysv1.QualysContainerSecurity) error {
	dsName := sensor.Name + "-runtime"
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: dsName, Namespace: sensor.Namespace}, ds); err == nil {
		if err := r.Delete(ctx, ds); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *QualysContainerSecurityReconciler) updateComponentStatuses(ctx context.Context, sensor *qualysv1.QualysContainerSecurity) error {
	containerSensorCfg := sensor.Spec.GetContainerSensor()
	sensor.Status.ContainerSensor = r.getDaemonSetComponentStatus(ctx, sensor.Name+"-container", sensor.Namespace, containerSensorCfg.Enabled)

	clusterSensorCfg := sensor.Spec.GetClusterSensor()
	sensor.Status.ClusterSensor = r.getDeploymentComponentStatus(ctx, sensor.Name+"-cluster", sensor.Namespace, clusterSensorCfg.Enabled)

	admissionCfg := sensor.Spec.GetAdmissionController()
	sensor.Status.AdmissionController = r.getDeploymentComponentStatus(ctx, sensor.Name+"-admission", sensor.Namespace, admissionCfg.Enabled)

	runtimeSensorCfg := sensor.Spec.GetRuntimeSensor()
	sensor.Status.RuntimeSensor = r.getDaemonSetComponentStatus(ctx, sensor.Name+"-runtime", sensor.Namespace, runtimeSensorCfg.Enabled)

	sensor.Status.ObservedGeneration = sensor.Generation

	allReady := isComponentReady(sensor.Status.ContainerSensor) &&
		isComponentReady(sensor.Status.ClusterSensor) &&
		isComponentReady(sensor.Status.AdmissionController) &&
		isComponentReady(sensor.Status.RuntimeSensor)

	if allReady {
		r.setCondition(sensor, qualysv1.ConditionTypeAvailable, metav1.ConditionTrue, "AllComponentsReady", "All enabled components are ready")
		r.setCondition(sensor, qualysv1.ConditionTypeProgressing, metav1.ConditionFalse, "DeploymentComplete", "Rollout complete")
		r.setCondition(sensor, qualysv1.ConditionTypeDegraded, metav1.ConditionFalse, "AllComponentsHealthy", "No degraded components")
	} else {
		r.setCondition(sensor, qualysv1.ConditionTypeAvailable, metav1.ConditionFalse, "ComponentsNotReady", "Some components are not ready")
		r.setCondition(sensor, qualysv1.ConditionTypeProgressing, metav1.ConditionTrue, "RollingOut", "Components rolling out")
	}

	return r.Status().Update(ctx, sensor)
}

func (r *QualysContainerSecurityReconciler) getDaemonSetComponentStatus(ctx context.Context, name, namespace string, enabled bool) *qualysv1.ComponentStatus {
	status := &qualysv1.ComponentStatus{Enabled: enabled}
	if !enabled {
		return status
	}

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ds); err != nil {
		status.Message = "DaemonSet not found"
		return status
	}

	status.ResourceName = ds.Name
	status.DesiredReplicas = ds.Status.DesiredNumberScheduled
	status.ReadyReplicas = ds.Status.NumberReady
	status.Ready = ds.Status.NumberReady == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0
	if status.Ready {
		status.Message = "All pods ready"
	} else {
		status.Message = fmt.Sprintf("%d/%d pods ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
	}
	return status
}

func (r *QualysContainerSecurityReconciler) getDeploymentComponentStatus(ctx context.Context, name, namespace string, enabled bool) *qualysv1.ComponentStatus {
	status := &qualysv1.ComponentStatus{Enabled: enabled}
	if !enabled {
		return status
	}

	deploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deploy); err != nil {
		status.Message = "Deployment not found"
		return status
	}

	status.ResourceName = deploy.Name
	status.DesiredReplicas = *deploy.Spec.Replicas
	status.ReadyReplicas = deploy.Status.ReadyReplicas
	status.Ready = deploy.Status.ReadyReplicas == *deploy.Spec.Replicas && *deploy.Spec.Replicas > 0
	if status.Ready {
		status.Message = "All replicas ready"
	} else {
		status.Message = fmt.Sprintf("%d/%d replicas ready", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas)
	}
	return status
}

func (r *QualysContainerSecurityReconciler) setCondition(sensor *qualysv1.QualysContainerSecurity, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: sensor.Generation,
		Reason:             reason,
		Message:            message,
	}
	qualysv1.SetCondition(&sensor.Status.Conditions, condition)
}

func (r *QualysContainerSecurityReconciler) validatePrivilegeModeFeatures(sensor *qualysv1.QualysContainerSecurity, cfg *qualysv1.ContainerSensorConfig) {
	if cfg.Scanning == nil {
		return
	}

	switch cfg.PrivilegeMode {
	case qualysv1.PrivilegeModeUnprivileged:
		if cfg.Scanning.EnableContainerScan {
			r.Recorder.Event(sensor, corev1.EventTypeWarning, "FeatureDisabled",
				"Container scanning requires 'minimal' or 'standard' privilege mode. Container scanning will be disabled in 'unprivileged' mode.")
		}
		if cfg.Scanning.EnableMalwareDetection {
			r.Recorder.Event(sensor, corev1.EventTypeWarning, "FeatureDisabled",
				"Malware detection requires 'standard' privilege mode. Malware detection will be disabled in 'unprivileged' mode.")
		}
		if cfg.Scanning.EnableSecretDetection {
			r.Recorder.Event(sensor, corev1.EventTypeWarning, "FeatureDisabled",
				"Secret detection in containers requires 'minimal' or 'standard' privilege mode. Only image-based secret detection is available in 'unprivileged' mode.")
		}
		r.Recorder.Event(sensor, corev1.EventTypeNormal, "UnprivilegedMode",
			"Running in unprivileged mode (Pod Security Standards: restricted). Only image vulnerability scanning via CRI API is available.")

	case qualysv1.PrivilegeModeMinimal:
		if cfg.Scanning.EnableMalwareDetection {
			r.Recorder.Event(sensor, corev1.EventTypeWarning, "FeatureDisabled",
				"Malware detection requires 'standard' privilege mode. Malware detection will be disabled in 'minimal' mode.")
		}
		r.Recorder.Event(sensor, corev1.EventTypeNormal, "MinimalMode",
			"Running in minimal privilege mode (Pod Security Standards: baseline). Image and container scanning available, no malware detection.")

	case qualysv1.PrivilegeModeStandard:
		r.Recorder.Event(sensor, corev1.EventTypeNormal, "StandardMode",
			"Running in standard privilege mode with specific capabilities (privileged: false). All scanning features available. This is the recommended configuration.")

	case qualysv1.PrivilegeModePrivileged:
		r.Recorder.Event(sensor, corev1.EventTypeWarning, "PrivilegedMode",
			"Running in privileged mode (privileged: true). Consider using 'standard' mode instead which provides the same functionality without privileged containers.")
	}
}

func (r *QualysContainerSecurityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qualysv1.QualysContainerSecurity{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Named("qualyscontainersecurity").
		Complete(r)
}

func buildContainerSensorArgs(cfg qualysv1.ContainerSensorConfig, runtimeName string) []string {
	args := []string{}
	if cfg.K8sMode {
		args = append(args, "--k8s-mode")
	}
	if runtimeName != "" {
		args = append(args, "--container-runtime", runtimeName)
	}
	switch cfg.Mode {
	case qualysv1.ContainerSensorModeCICD:
		args = append(args, "--cicd-deployed-sensor")
	case qualysv1.ContainerSensorModeRegistry:
		args = append(args, "--registry-sensor")
	}
	if cfg.Logging != nil {
		args = append(args, "--log-level", fmt.Sprintf("%d", cfg.Logging.LogLevel))
	}
	if cfg.Storage != nil && !cfg.Storage.UsePersistentStorage {
		args = append(args, "--sensor-without-persistent-storage")
	}
	if cfg.Scanning != nil {
		policy := cfg.Scanning.ScanningPolicy
		if policy == "" {
			policy = qualysv1.ScanningPolicyDynamicWithStaticFallback
		}
		args = append(args, "--scanning-policy", string(policy))

		if cfg.Scanning.EnableScaScan {
			args = append(args, "--perform-sca-scan")
		}
	}
	args = append(args, cfg.ExtraArgs...)
	return args
}

func buildContainerSensorEnvVars(secretName, serverUri string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "CUSTOMERID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "CUSTOMER_ID"}}},
		{Name: "ACTIVATIONID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}, Key: "ACTIVATION_ID"}}},
		{Name: "POD_URL", Value: serverUri},
		{Name: "QUALYS_SCANNING_CONTAINER_LAUNCH_TIMEOUT", Value: "10"},
		{Name: "QUALYS_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
		{Name: "QUALYS_POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
		{Name: "QUALYS_SENSOR_HOST_IP", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"}}},
	}
}

func buildContainerSensorVolumes(socketPath string, rt platform.ContainerRuntime, privilegeMode qualysv1.PrivilegeMode) ([]corev1.VolumeMount, []corev1.Volume) {
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	directory := corev1.HostPathDirectory
	socket := corev1.HostPathSocket
	fileType := corev1.HostPathFile

	switch privilegeMode {
	case qualysv1.PrivilegeModeUnprivileged:
		volumeMounts := []corev1.VolumeMount{
			{Name: "runtime-socket", MountPath: socketPath, ReadOnly: true},
			{Name: "sensor-data", MountPath: "/usr/local/qualys/qpa/data"},
			{Name: "tmp", MountPath: "/tmp"},
		}
		volumes := []corev1.Volume{
			{Name: "runtime-socket", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: socketPath, Type: &socket}}},
			{Name: "sensor-data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		}
		return volumeMounts, volumes

	case qualysv1.PrivilegeModeMinimal:
		volumeMounts := []corev1.VolumeMount{
			{Name: "runtime-socket", MountPath: socketPath},
			{Name: "host-root", MountPath: "/host", ReadOnly: true},
			{Name: "sensor-data", MountPath: "/usr/local/qualys/qpa/data"},
			{Name: "tmp", MountPath: "/tmp"},
		}
		volumes := []corev1.Volume{
			{Name: "runtime-socket", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: socketPath, Type: &socket}}},
			{Name: "host-root", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/", Type: &directory}}},
			{Name: "sensor-data", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/usr/local/qualys/sensor/data", Type: &directoryOrCreate}}},
			{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		}
		return volumeMounts, volumes

	case qualysv1.PrivilegeModePrivileged:
		volumeMounts := []corev1.VolumeMount{
			{Name: "socket-volume", MountPath: socketPath, ReadOnly: true},
			{Name: "persistent-volume", MountPath: "/usr/local/qualys/qpa/data"},
			{Name: "agent-volume", MountPath: "/usr/local/qualys/qpa/data/conf/agent-data"},
		}
		volumes := []corev1.Volume{
			{Name: "socket-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: socketPath, Type: &socket}}},
			{Name: "persistent-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/usr/local/qualys/sensor/data", Type: &directoryOrCreate}}},
			{Name: "agent-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/qualys", Type: &directoryOrCreate}}},
		}
		if rt == platform.RuntimeCRIO {
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{Name: "container-storage", MountPath: "/var/lib/containers/storage", ReadOnly: true},
				corev1.VolumeMount{Name: "storage-config-volume", MountPath: "/etc/containers/storage.conf", ReadOnly: true},
			)
			volumes = append(volumes,
				corev1.Volume{Name: "container-storage", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/containers/storage"}}},
				corev1.Volume{Name: "storage-config-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/containers/storage.conf", Type: &fileType}}},
			)
		}
		return volumeMounts, volumes

	default:
		volumeMounts := []corev1.VolumeMount{
			{Name: "socket-volume", MountPath: socketPath, ReadOnly: true},
			{Name: "persistent-volume", MountPath: "/usr/local/qualys/qpa/data"},
			{Name: "agent-volume", MountPath: "/usr/local/qualys/qpa/data/conf/agent-data"},
		}
		volumes := []corev1.Volume{
			{Name: "socket-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: socketPath, Type: &socket}}},
			{Name: "persistent-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/usr/local/qualys/sensor/data", Type: &directoryOrCreate}}},
			{Name: "agent-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/qualys", Type: &directoryOrCreate}}},
		}
		if rt == platform.RuntimeCRIO {
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{Name: "container-storage", MountPath: "/var/lib/containers/storage", ReadOnly: true},
				corev1.VolumeMount{Name: "storage-config-volume", MountPath: "/etc/containers/storage.conf", ReadOnly: true},
			)
			volumes = append(volumes,
				corev1.Volume{Name: "container-storage", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/containers/storage"}}},
				corev1.Volume{Name: "storage-config-volume", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/containers/storage.conf", Type: &fileType}}},
			)
		}
		return volumeMounts, volumes
	}
}

func getRuntimeName(rt platform.ContainerRuntime) string {
	switch rt {
	case platform.RuntimeContainerd:
		return "containerd"
	case platform.RuntimeCRIO:
		return "cri-o"
	case platform.RuntimeDocker:
		return "docker"
	default:
		return ""
	}
}

func getSocketPath(runtimeConfig qualysv1.ContainerRuntimeConfig, rt platform.ContainerRuntime) string {
	if runtimeConfig.SocketPaths == nil {
		switch rt {
		case platform.RuntimeCRIO:
			return "/var/run/crio/crio.sock"
		case platform.RuntimeDocker:
			return "/var/run/docker.sock"
		default:
			return "/var/run/containerd/containerd.sock"
		}
	}

	switch rt {
	case platform.RuntimeCRIO:
		if runtimeConfig.SocketPaths.CRIO != "" {
			return runtimeConfig.SocketPaths.CRIO
		}
		return "/var/run/crio/crio.sock"
	case platform.RuntimeDocker:
		if runtimeConfig.SocketPaths.Docker != "" {
			return runtimeConfig.SocketPaths.Docker
		}
		return "/var/run/docker.sock"
	default:
		if runtimeConfig.SocketPaths.Containerd != "" {
			return runtimeConfig.SocketPaths.Containerd
		}
		return "/var/run/containerd/containerd.sock"
	}
}

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }
func strPtr(s string) *string { return &s }

func isComponentReady(status *qualysv1.ComponentStatus) bool {
	if status == nil || !status.Enabled {
		return true
	}
	return status.Ready
}
