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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	qualysv1 "github.com/nelssec/qualys-nanny/api/v1"
	"github.com/nelssec/qualys-nanny/internal/credentials"
)

const (
	RequeueIntervalDefault = 5 * time.Minute
	RequeueIntervalError   = 30 * time.Second
)

type QualysPlatformConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=qualys.io,resources=qualysplatformconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=qualys.io,resources=qualysplatformconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=qualys.io,resources=qualysplatformconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *QualysPlatformConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	config := &qualysv1.QualysPlatformConfig{}
	err := r.Get(ctx, req.NamespacedName, config)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("QualysPlatformConfig resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get QualysPlatformConfig")
		return ctrl.Result{}, err
	}

	if config.Status.Conditions == nil {
		config.Status.Conditions = []metav1.Condition{}
	}

	credResolver := credentials.NewResolver(r.Client)
	creds, err := credResolver.Resolve(ctx, config)
	if err != nil {
		log.Error(err, "Failed to resolve credentials")
		r.setCredentialsCondition(config, metav1.ConditionFalse, "CredentialError", err.Error())
		if updateErr := r.Status().Update(ctx, config); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		r.Recorder.Event(config, corev1.EventTypeWarning, "CredentialError", err.Error())
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	if creds.ActivationID == "" || creds.CustomerID == "" {
		log.Info("Credentials are empty")
		r.setCredentialsCondition(config, metav1.ConditionFalse, "CredentialsEmpty", "Activation ID or Customer ID is empty")
		if updateErr := r.Status().Update(ctx, config); updateErr != nil {
			log.Error(updateErr, "Failed to update status")
		}
		r.Recorder.Event(config, corev1.EventTypeWarning, "CredentialsEmpty", "Activation ID or Customer ID is empty")
		return ctrl.Result{RequeueAfter: RequeueIntervalError}, nil
	}

	r.setCredentialsCondition(config, metav1.ConditionTrue, "CredentialsReady", "Credentials are available")
	config.Status.CredentialSource = creds.Source
	now := metav1.Now()
	config.Status.LastValidated = &now
	config.Status.ObservedGeneration = config.Generation

	if err := r.Status().Update(ctx, config); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(config, corev1.EventTypeNormal, "CredentialsValidated", "Credentials validated successfully")
	return ctrl.Result{RequeueAfter: RequeueIntervalDefault}, nil
}

func (r *QualysPlatformConfigReconciler) setCredentialsCondition(config *qualysv1.QualysPlatformConfig, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               qualysv1.ConditionTypeCredentialsReady,
		Status:             status,
		ObservedGeneration: config.Generation,
		Reason:             reason,
		Message:            message,
	}
	qualysv1.SetCondition(&config.Status.Conditions, condition)
}

func (r *QualysPlatformConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&qualysv1.QualysPlatformConfig{}).
		Named("qualysplatformconfig").
		Complete(r)
}
