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

type QualysCloudAgentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	DynamicClient dynamic.Interface
}

// +kubebuilder:rbac:groups=qualys.qualys.io,resources=qualyscloudagents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=qualys.qualys.io,resources=qualyscloudagents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=qualys.qualys.io,resources=qualyscloudagents/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.openshift.io,resources=securitycontextconstraints,verbs=get;list;watch;create;update;patch;delete;use

func (r *QualysCloudAgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	agent := &qualysv1alpha1.QualysCloudAgent{}
	err := r.Get(ctx, req.NamespacedName, agent)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("QualysCloudAgent resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get QualysCloudAgent")
		return ctrl.Result{}, err
	}

	if agent.Status.Conditions == nil {
		agent.Status.Conditions = []metav1.Condition{}
	}

	platformConfig := &qualysv1alpha1.QualysPlatformConfig{}
	err = r.Get(ctx, types.NamespacedName{Name: agent.Spec.PlatformConfigRef.Name}, platformConfig)
	if err != nil {
		log.Error(err, "Failed to get QualysPlatformConfig", "name", agent.Spec.PlatformConfigRef.Name)
		r.setCondition(agent, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "PlatformConfigNotFound", err.Error())
		if updateErr := r.Status().Update(ctx, agent); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		r.Recorder.Event(agent, corev1.EventTypeWarning, "PlatformConfigNotFound", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	credCond := qualysv1alpha1.GetCondition(platformConfig.Status.Conditions, qualysv1alpha1.ConditionTypeCredentialsReady)
	if credCond == nil || credCond.Status != metav1.ConditionTrue {
		log.Info("Credentials not ready, waiting")
		r.setCondition(agent, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "CredentialsNotReady", "Waiting for QualysPlatformConfig credentials")
		if updateErr := r.Status().Update(ctx, agent); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	credResolver := credentials.NewResolver(r.Client)
	secretRef, err := credResolver.GetSecretRef(platformConfig)
	if err != nil {
		log.Error(err, "Failed to get secret reference")
		r.setCondition(agent, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "SecretRefError", err.Error())
		if updateErr := r.Status().Update(ctx, agent); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	r.setCondition(agent, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionTrue, "Reconciling", "Creating/updating resources")

	serviceAccountName := agent.Name + "-sa"
	configMapName := agent.Name + "-config"
	clusterRoleName := agent.Name + "-role"
	clusterRoleBindingName := agent.Name + "-rolebinding"
	sccName := agent.Name + "-scc"

	if err := r.reconcileServiceAccount(ctx, agent, serviceAccountName); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileClusterRole(ctx, clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileClusterRoleBinding(ctx, agent, clusterRoleBindingName, serviceAccountName, clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}
	if platform.IsOpenShift(ctx) {
		if err := r.reconcileSCC(ctx, agent, sccName, serviceAccountName); err != nil {
			log.Error(err, "Failed to reconcile SecurityContextConstraints", "scc", sccName)
			r.Recorder.Event(agent, corev1.EventTypeWarning, "SCCReconcileFailed", err.Error())
		}
	}

	agentConfig := agent.Spec.GetConfig()
	if err := r.reconcileConfigMap(ctx, agent, configMapName, platformConfig, agentConfig); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileDaemonSet(ctx, agent, configMapName, secretRef.Name, serviceAccountName); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateStatusFromDaemonSet(ctx, agent); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: RequeueIntervalDefault}, nil
}

func (r *QualysCloudAgentReconciler) reconcileServiceAccount(ctx context.Context, agent *qualysv1alpha1.QualysCloudAgent, name string) error {
	sa := resources.BuildServiceAccount(name, agent.Namespace, "cloud-agent")
	if err := controllerutil.SetControllerReference(agent, sa, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, sa)
		}
		return err
	}

	// Update labels if needed
	existing.Labels = sa.Labels
	return r.Update(ctx, existing)
}

func (r *QualysCloudAgentReconciler) reconcileClusterRole(ctx context.Context, name string) error {
	role := resources.BuildCloudAgentClusterRole(name)
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

func (r *QualysCloudAgentReconciler) reconcileClusterRoleBinding(ctx context.Context, agent *qualysv1alpha1.QualysCloudAgent, name, serviceAccountName, clusterRoleName string) error {
	binding := resources.BuildCloudAgentClusterRoleBinding(name, agent.Namespace, serviceAccountName, clusterRoleName)
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

func (r *QualysCloudAgentReconciler) reconcileSCC(ctx context.Context, agent *qualysv1alpha1.QualysCloudAgent, name, serviceAccountName string) error {
	if r.DynamicClient == nil {
		return fmt.Errorf("dynamic client not configured")
	}
	scc := resources.BuildCloudAgentSCC(name, agent.Namespace, serviceAccountName)
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

func (r *QualysCloudAgentReconciler) reconcileConfigMap(ctx context.Context, agent *qualysv1alpha1.QualysCloudAgent, name string, platformConfig *qualysv1alpha1.QualysPlatformConfig, agentConfig qualysv1alpha1.CloudAgentConfig) error {
	cm := resources.BuildCloudAgentConfigMap(name, agent.Namespace, platformConfig, agentConfig)
	if err := controllerutil.SetControllerReference(agent, cm, r.Scheme); err != nil {
		return err
	}
	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: agent.Namespace}, existing)
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

func (r *QualysCloudAgentReconciler) reconcileDaemonSet(ctx context.Context, agent *qualysv1alpha1.QualysCloudAgent, configMapName, secretName, serviceAccountName string) error {
	ds := resources.BuildCloudAgentDaemonSet(agent, configMapName, secretName, serviceAccountName)
	if err := controllerutil.SetControllerReference(agent, ds, r.Scheme); err != nil {
		return err
	}
	existing := &appsv1.DaemonSet{}
	err := r.Get(ctx, types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.Recorder.Event(agent, corev1.EventTypeNormal, "DaemonSetCreated", "Created DaemonSet "+agent.Name)
			return r.Create(ctx, ds)
		}
		return err
	}

	// Update spec
	existing.Spec = ds.Spec
	existing.Labels = ds.Labels
	r.Recorder.Event(agent, corev1.EventTypeNormal, "DaemonSetUpdated", "Updated DaemonSet "+agent.Name)
	return r.Update(ctx, existing)
}

func (r *QualysCloudAgentReconciler) updateStatusFromDaemonSet(ctx context.Context, agent *qualysv1alpha1.QualysCloudAgent) error {
	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}, ds); err != nil {
		return err
	}

	agent.Status.DesiredNumberScheduled = ds.Status.DesiredNumberScheduled
	agent.Status.CurrentNumberScheduled = ds.Status.CurrentNumberScheduled
	agent.Status.NumberReady = ds.Status.NumberReady
	agent.Status.NumberAvailable = ds.Status.NumberAvailable
	agent.Status.UpdatedNumberScheduled = ds.Status.UpdatedNumberScheduled
	agent.Status.NumberMisscheduled = ds.Status.NumberMisscheduled
	agent.Status.DaemonSetName = ds.Name
	agent.Status.ObservedGeneration = agent.Generation

	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
		r.setCondition(agent, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionTrue, "DaemonSetReady", "All pods are ready")
		r.setCondition(agent, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionFalse, "DeploymentComplete", "Rollout complete")
		r.setCondition(agent, qualysv1alpha1.ConditionTypeDegraded, metav1.ConditionFalse, "AllPodsHealthy", "No degraded pods")
	} else if ds.Status.DesiredNumberScheduled == 0 {
		r.setCondition(agent, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "NoPods", "No pods scheduled")
		r.setCondition(agent, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionTrue, "WaitingForPods", "Waiting for pods to be scheduled")
	} else {
		r.setCondition(agent, qualysv1alpha1.ConditionTypeAvailable, metav1.ConditionFalse, "NotAllReady", fmt.Sprintf("%d/%d pods ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled))
		r.setCondition(agent, qualysv1alpha1.ConditionTypeProgressing, metav1.ConditionTrue, "RollingOut", "DaemonSet rollout in progress")
		if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
			r.setCondition(agent, qualysv1alpha1.ConditionTypeDegraded, metav1.ConditionTrue, "PodsNotReady", "Some pods are not ready")
		}
	}

	return r.Status().Update(ctx, agent)
}

func (r *QualysCloudAgentReconciler) setCondition(agent *qualysv1alpha1.QualysCloudAgent, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: agent.Generation,
		Reason:             reason,
		Message:            message,
	}
	qualysv1alpha1.SetCondition(&agent.Status.Conditions, condition)
}

func (r *QualysCloudAgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qualysv1alpha1.QualysCloudAgent{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.ConfigMap{}).
		Named("qualyscloudagent").
		Complete(r)
}
