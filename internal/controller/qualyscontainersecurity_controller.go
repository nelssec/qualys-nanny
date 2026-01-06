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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	qualysv1alpha1 "github.com/nelssec/qualys-nanny/api/v1alpha1"
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

// +kubebuilder:rbac:groups=qualys.qualys.io,resources=qualyscontainersecurities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=qualys.qualys.io,resources=qualyscontainersecurities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=qualys.qualys.io,resources=qualyscontainersecurities/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts;configmaps;secrets;nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

func (r *QualysContainerSecurityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sensor := &qualysv1alpha1.QualysContainerSecurity{}
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

	platformConfig := &qualysv1alpha1.QualysPlatformConfig{}
	err = r.Get(ctx, types.NamespacedName{Name: sensor.Spec.PlatformConfigRef.Name}, platformConfig)
	if err != nil {
		log.Error(err, "Failed to get QualysPlatformConfig", "name", sensor.Spec.PlatformConfigRef.Name)
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "PlatformConfigNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, sensor); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		r.Recorder.Event(sensor, corev1.EventTypeWarning, "PlatformConfigNotFound", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	credCond := qualysv1alpha1.GetCondition(platformConfig.Status.Conditions, qualysv1alpha1.ConditionTypeCredentialsReady)
	if credCond == nil || credCond.Status != metav1.ConditionTrue {
		log.Info("Credentials not ready, waiting")
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "CredentialsNotReady", "Waiting for QualysPlatformConfig credentials")
		if updateErr := r.Status().Update(ctx, sensor); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	credResolver := credentials.NewResolver(r.Client)
	secretRef, err := credResolver.GetSecretRef(platformConfig)
	if err != nil {
		log.Error(err, "Failed to get secret reference")
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "SecretRefError", err.Error())
		if updateErr := r.Status().Update(ctx, sensor); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	detectedRuntime := r.detectContainerRuntime(ctx, sensor)
	sensor.Status.DetectedRuntime = string(detectedRuntime)
	r.setCondition(sensor, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionTrue, "Reconciling", "Creating/updating resources")

	serviceAccountName := sensor.Name + "-sa"
	configMapName := sensor.Name + "-config"
	clusterRoleName := sensor.Name + "-role"
	clusterRoleBindingName := sensor.Name + "-rolebinding"
	sccName := sensor.Name + "-scc"

	if err := r.reconcileServiceAccount(ctx, sensor, serviceAccountName); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileClusterRole(ctx, clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileClusterRoleBinding(ctx, sensor, clusterRoleBindingName, serviceAccountName, clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}
	if platform.IsOpenShift(ctx) {
		if err := r.reconcileSCC(ctx, sensor, sccName, serviceAccountName); err != nil {
			log.Error(err, "Failed to reconcile SecurityContextConstraints", "scc", sccName)
			r.Recorder.Event(sensor, corev1.EventTypeWarning, "SCCReconcileFailed", err.Error())
		}
	}

	sensorConfig := sensor.Spec.GetSensorConfig()
	if err := r.reconcileConfigMap(ctx, sensor, configMapName, platformConfig, sensorConfig); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileDaemonSet(ctx, sensor, configMapName, secretRef.Name, serviceAccountName, detectedRuntime); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateStatusFromDaemonSet(ctx, sensor); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: RequeueIntervalDefault}, nil
}

func (r *QualysContainerSecurityReconciler) detectContainerRuntime(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity) platform.ContainerRuntime {
	runtimeConfig := sensor.Spec.GetContainerRuntime()
	if runtimeConfig.Type != qualysv1alpha1.ContainerRuntimeAuto {
		switch runtimeConfig.Type {
		case qualysv1alpha1.ContainerRuntimeContainerd:
			return platform.RuntimeContainerd
		case qualysv1alpha1.ContainerRuntimeCRIO:
			return platform.RuntimeCRIO
		case qualysv1alpha1.ContainerRuntimeDocker:
			return platform.RuntimeDocker
		}
	}

	if platform.IsOpenShift(ctx) {
		return platform.RuntimeCRIO
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList, client.Limit(1)); err == nil && len(nodeList.Items) > 0 {
		node := nodeList.Items[0]
		return platform.DetectContainerRuntime(node.Status.NodeInfo.ContainerRuntimeVersion)
	}

	return platform.RuntimeUnknown
}

func (r *QualysContainerSecurityReconciler) reconcileServiceAccount(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity, name string) error {
	sa := resources.BuildServiceAccount(name, sensor.Namespace, "container-sensor")

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

func (r *QualysContainerSecurityReconciler) reconcileClusterRole(ctx context.Context, name string) error {
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

func (r *QualysContainerSecurityReconciler) reconcileClusterRoleBinding(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity, name, serviceAccountName, clusterRoleName string) error {
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

func (r *QualysContainerSecurityReconciler) reconcileSCC(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity, name, serviceAccountName string) error {
	if r.DynamicClient == nil {
		return fmt.Errorf("dynamic client not configured")
	}

	scc := resources.BuildContainerSensorSCC(name, sensor.Namespace, serviceAccountName)

	_, err := r.DynamicClient.Resource(platform.SCCGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = r.DynamicClient.Resource(platform.SCCGVR).Create(ctx, scc, metav1.CreateOptions{})
			return err
		}
		return err
	}

	_, err = r.DynamicClient.Resource(platform.SCCGVR).Update(ctx, scc, metav1.UpdateOptions{})
	return err
}

func (r *QualysContainerSecurityReconciler) reconcileConfigMap(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity, name string, platformConfig *qualysv1alpha1.QualysPlatformConfig, sensorConfig qualysv1alpha1.SensorConfig) error {
	cm := resources.BuildContainerSensorConfigMap(name, sensor.Namespace, platformConfig, sensorConfig)

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

func (r *QualysContainerSecurityReconciler) reconcileDaemonSet(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity, configMapName, secretName, serviceAccountName string, rt platform.ContainerRuntime) error {
	ds := r.buildContainerSensorDaemonSet(sensor, configMapName, secretName, serviceAccountName, rt)

	if err := controllerutil.SetControllerReference(sensor, ds, r.Scheme); err != nil {
		return err
	}

	existing := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: sensor.Name, Namespace: sensor.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(sensor, corev1.EventTypeNormal, "DaemonSetCreated", "Created DaemonSet "+sensor.Name)
			return r.Create(ctx, ds)
		}
		return err
	}

	existing.Spec = ds.Spec
	existing.Labels = ds.Labels
	r.Recorder.Event(sensor, corev1.EventTypeNormal, "DaemonSetUpdated", "Updated DaemonSet "+sensor.Name)
	return r.Update(ctx, existing)
}

func (r *QualysContainerSecurityReconciler) buildContainerSensorDaemonSet(sensor *qualysv1alpha1.QualysContainerSecurity, configMapName, secretName, serviceAccountName string, rt platform.ContainerRuntime) *appsv1.DaemonSet {
	image := sensor.Spec.GetImage()
	scheduling := sensor.Spec.GetScheduling()
	updateStrategy := sensor.Spec.GetUpdateStrategy()

	labels := map[string]string{
		"app.kubernetes.io/name":       "qualys-container-sensor",
		"app.kubernetes.io/instance":   sensor.Name,
		"app.kubernetes.io/managed-by": "qualys-nanny",
		"app.kubernetes.io/component":  "container-sensor",
	}

	runtimeConfig := sensor.Spec.GetContainerRuntime()
	socketPath := getSocketPath(runtimeConfig, rt)

	maxUnavailable := parseIntOrString(updateStrategy.RollingUpdate.MaxUnavailable)

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sensor.Name,
			Namespace: sensor.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.DaemonSetUpdateStrategyType(updateStrategy.Type),
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
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
					Containers: []corev1.Container{
						{
							Name:            resources.ContainerSensorContainerName,
							Image:           fmt.Sprintf("%s:%s", image.Repository, image.Tag),
							ImagePullPolicy: image.PullPolicy,
							Env: []corev1.EnvVar{
								{
									Name: "ACTIVATION_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
											Key:                  "ACTIVATION_ID",
										},
									},
								},
								{
									Name: "CUSTOMER_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
											Key:                  "CUSTOMER_ID",
										},
									},
								},
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
								{
									Name:  "RUNTIME_SOCKET",
									Value: socketPath,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "runtime-socket", MountPath: socketPath},
								{Name: "var-run", MountPath: "/var/run"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "runtime-socket",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: socketPath,
								},
							},
						},
						{
							Name: "var-run",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/run",
								},
							},
						},
					},
					ImagePullSecrets: image.PullSecrets,
				},
			},
		},
	}

	return ds
}

func (r *QualysContainerSecurityReconciler) updateStatusFromDaemonSet(ctx context.Context, sensor *qualysv1alpha1.QualysContainerSecurity) error {
	ds := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: sensor.Name, Namespace: sensor.Namespace}, ds)
	if err != nil {
		return err
	}

	sensor.Status.DesiredNumberScheduled = ds.Status.DesiredNumberScheduled
	sensor.Status.CurrentNumberScheduled = ds.Status.CurrentNumberScheduled
	sensor.Status.NumberReady = ds.Status.NumberReady
	sensor.Status.NumberAvailable = ds.Status.NumberAvailable
	sensor.Status.UpdatedNumberScheduled = ds.Status.UpdatedNumberScheduled
	sensor.Status.NumberMisscheduled = ds.Status.NumberMisscheduled
	sensor.Status.DaemonSetName = ds.Name
	sensor.Status.ObservedGeneration = sensor.Generation

	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionTrue, "DaemonSetReady", "All pods are ready")
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionFalse, "DeploymentComplete", "Rollout complete")
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeDegraded, metav1.ConditionFalse, "AllPodsHealthy", "No degraded pods")
	} else if ds.Status.DesiredNumberScheduled == 0 {
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "NoPods", "No pods scheduled")
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionTrue, "WaitingForPods", "Waiting for pods to be scheduled")
	} else {
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "NotAllReady", fmt.Sprintf("%d/%d pods ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled))
		r.setCondition(sensor, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionTrue, "RollingOut", "DaemonSet rollout in progress")
		if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			r.setCondition(sensor, qualysv1alpha1.ConditionTypeDegraded, metav1.ConditionTrue, "PodsNotReady", "Some pods are not ready")
		}
	}

	return r.Status().Update(ctx, sensor)
}

func (r *QualysContainerSecurityReconciler) setCondition(sensor *qualysv1alpha1.QualysContainerSecurity, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: sensor.Generation,
		Reason:             reason,
		Message:            message,
	}
	qualysv1alpha1.SetCondition(&sensor.Status.Conditions, condition)
}

func (r *QualysContainerSecurityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qualysv1alpha1.QualysContainerSecurity{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Named("qualyscontainersecurity").
		Complete(r)
}

func parseIntOrString(val string) intstr.IntOrString {
	return intstr.FromString(val)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func getSocketPath(runtimeConfig qualysv1alpha1.ContainerRuntimeConfig, rt platform.ContainerRuntime) string {
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
