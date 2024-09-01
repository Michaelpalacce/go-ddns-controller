/*
Copyright 2024.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	notifierConditions "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/notifier/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/notifiers"
	"github.com/go-logr/logr"
)

// NotifierReconciler reconciles a Notifier object
type NotifierReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	NotifierFactory func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error)
}

// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=notifiers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=notifiers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=notifiers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers,verbs=get;list;watch;patch

// Reconcile will reconcile the Notifier object
func (r *NotifierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	notifier := &ddnsv1alpha1.Notifier{}

	if err := r.Get(ctx, req.NamespacedName, notifier); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Notifier triggered")

	notifierClient, err := r.FetchNotifier(ctx, req, notifier, log)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to fetch notifier: %w", err)
	}

	if !notifier.Status.IsReady {
		if err := r.MarkAsReady(ctx, notifier, notifierClient, log); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to mark Notifier as ready: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	providers := &ddnsv1alpha1.ProviderList{}
	if err := r.List(ctx, providers); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to list Providers: %w", err)
	}

	filteredProviders := []ddnsv1alpha1.Provider{}
	for _, provider := range providers.Items {
		for _, ref := range provider.Spec.NotifierRefs {
			if ref.Name == req.Name {
				filteredProviders = append(filteredProviders, provider)
				break
			}
		}
	}

	for _, provider := range filteredProviders {
		if provider.Annotations != nil {
			if err = r.NotifyOfChange(ctx, req, &provider, notifier, notifierClient, log); err != nil {
				return ctrl.Result{}, fmt.Errorf("unabel to notify of change: %w", err)
			}
		}
	}

	if err := r.PatchStatus(ctx, notifier, r.updateObservedGeneration(notifier, notifier.GetGeneration()), log); err != nil {
		return ctrl.Result{}, fmt.Errorf("uanbel to update Notifier status: %w", err)
	}

	return ctrl.Result{}, nil
}

// MarkAsReady marks the Notifier as ready
// Ready means that the Notifier has been successfully created and a greeting message has been sent
func (r *NotifierReconciler) MarkAsReady(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	notifierClient notifiers.Notifier,
	log logr.Logger,
) (err error) {
	condition := metav1.Condition{
		Type:   notifierConditions.ClientConditionType,
		Reason: notifierConditions.ClientCommunication,
	}

	if err = notifierClient.SendGreetings(notifier); err != nil {
		condition.Message = fmt.Sprintf("unable to send greetings: %s", err)
		condition.Status = metav1.ConditionFalse

		_ = r.UpdateConditions(ctx, notifier, condition, log)
		return fmt.Errorf("unable to send greetings: %w", err)
	}

	condition.Message = "Communications established"
	condition.Status = metav1.ConditionTrue

	_ = r.UpdateConditions(ctx, notifier, condition, log)

	if err := r.PatchStatus(ctx, notifier, r.updateIsReady(notifier, true), log); err != nil {
		return fmt.Errorf("unable to mark Notifier as ready: %w", err)
	}

	return nil
}

// NotifyOfChange sends a notification to the notifierClient
// We need to first update the annotation of the Provider with the new IP, then send the notification
// this is done to avoid issues with the resouceVersion of the Provider object
func (r *NotifierReconciler) NotifyOfChange(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	notifier *ddnsv1alpha1.Notifier,
	notifierClient notifiers.Notifier,
	log logr.Logger,
) error {
	annotation := fmt.Sprintf("%s/%s_%s", ddnsv1alpha1.GroupVersion.Group, req.Name, req.Namespace)
	if value, ok := provider.Annotations[annotation]; ok && value == provider.Status.ProviderIP {
		log.Info("Provider IP has not changed", "IP", provider.Status.ProviderIP)
		return nil
	}

	if provider.Status.ProviderIP == "" {
		log.Info("Provider IP is empty")
		return nil
	}

	log.Info("Provider IP changed", "IP", provider.Status.ProviderIP)

	var message string

	if provider.Status.ProviderIP == provider.Status.PublicIP {
		message = fmt.Sprintf("Provider IP (%s) in sync with Public IP. From provider: (%s).", provider.Status.ProviderIP, provider.Name)
	} else {
		message = fmt.Sprintf("Provider IP (%s) out of sync with Public IP (%s). From provider: (%s).", provider.Status.ProviderIP, provider.Status.PublicIP, provider.Name)
	}

	patch := client.MergeFrom(provider.DeepCopy())
	provider.Annotations[annotation] = provider.Status.ProviderIP

	if err := notifierClient.SendNotification(message); err != nil {
		log.Error(err, "unable to send notification")

		if err := r.PatchStatus(ctx, notifier, r.updateIsReady(notifier, false), log); err != nil {
			log.Error(err, "unable to mark Notifier as not ready")
		}

		condition := metav1.Condition{
			Type:    notifierConditions.ClientConditionType,
			Reason:  notifierConditions.ClientCommunication,
			Message: fmt.Sprintf("unable to send notification: %s", err),
			Status:  metav1.ConditionFalse,
		}

		if err := r.UpdateConditions(ctx, notifier, condition, log); err != nil {
			log.Error(err, "unable to update Notifier conditions")
		}

		return err
	}

	if err := r.Patch(ctx, provider, patch); err != nil {
		log.Error(err, "unable to update Provider annotation")
		return err
	}

	return nil
}

func (r *NotifierReconciler) FetchNotifier(
	ctx context.Context,
	req ctrl.Request,
	notifier *ddnsv1alpha1.Notifier,
	log logr.Logger,
) (notifiers.Notifier, error) {
	var (
		err     error
		message string
		status  metav1.ConditionStatus
	)

	configMap, err := r.FetchConfig(ctx, req, notifier, log)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch ConfigMap: %w", err)
	}

	secret, err := r.FetchSecret(ctx, req, notifier, log)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch Secret: %w", err)
	}

	notifierClient, err := r.NotifierFactory(notifier, secret, configMap)
	if err != nil {
		message = fmt.Sprintf("could not create client: %s", err)
		status = metav1.ConditionFalse
	} else {
		message = "Client created"
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:    notifierConditions.ClientConditionType,
		Reason:  notifierConditions.ClientCreated,
		Message: message,
		Status:  status,
	}

	_ = r.UpdateConditions(ctx, notifier, condition, log)

	return notifierClient, err
}

func (r *NotifierReconciler) FetchConfig(
	ctx context.Context,
	req ctrl.Request,
	notifier *ddnsv1alpha1.Notifier,
	log logr.Logger,
) (*corev1.ConfigMap, error) {
	var (
		configMap *corev1.ConfigMap
		err       error
		message   string
		status    metav1.ConditionStatus
	)
	configMap = &corev1.ConfigMap{}

	if err = r.Get(ctx, types.NamespacedName{Name: notifier.Spec.ConfigMap, Namespace: req.Namespace}, configMap); err != nil {
		message = fmt.Sprintf("ConfigMap %s not found", notifier.Spec.ConfigMap)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("ConfigMap %s found", notifier.Spec.ConfigMap)
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:    notifierConditions.ConfigMapConditionType,
		Reason:  notifierConditions.ConfigMapFound,
		Message: message,
		Status:  status,
	}

	_ = r.UpdateConditions(ctx, notifier, condition, log)

	return configMap, err
}

func (r *NotifierReconciler) FetchSecret(
	ctx context.Context,
	req ctrl.Request,
	notifier *ddnsv1alpha1.Notifier,
	log logr.Logger,
) (*corev1.Secret, error) {
	var (
		err     error
		message string
		status  metav1.ConditionStatus
		secret  *corev1.Secret
	)

	secret = &corev1.Secret{}
	if err = r.Get(ctx, types.NamespacedName{Name: notifier.Spec.SecretName, Namespace: req.Namespace}, secret); err != nil {
		message = fmt.Sprintf("Secret %s not found", notifier.Spec.SecretName)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("Secret %s found", notifier.Spec.SecretName)
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:    notifierConditions.SecretConditionType,
		Reason:  notifierConditions.SecretFound,
		Message: message,
		Status:  status,
	}

	_ = r.UpdateConditions(ctx, notifier, condition, log)

	return secret, nil
}

func (r *NotifierReconciler) UpdateConditions(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	condition metav1.Condition,
	log logr.Logger,
) error {
	return r.PatchStatus(ctx, notifier, func() bool {
		condition.ObservedGeneration = notifier.GetGeneration()

		return meta.SetStatusCondition(&notifier.Status.Conditions, condition)
	}, log)
}

func (r *NotifierReconciler) PatchStatus(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	apply func() bool,
	log logr.Logger,
) error {
	patch := client.MergeFrom(notifier.DeepCopy())
	if apply() {
		if err := r.Status().Patch(ctx, notifier, patch); err != nil {
			log.Error(err, "unable to patch status")
			return err
		}
	}

	return nil
}

// findObjectsForProvider returns a list of requests for Notifiers that are referenced by Providers
// providers have a `.spec.notifierRefs.*` field that references a Notifier
func (r *NotifierReconciler) findObjectsForProvider(ctx context.Context, provider client.Object) []reconcile.Request {
	notifierRefs := provider.(*ddnsv1alpha1.Provider).Spec.NotifierRefs

	requests := make([]reconcile.Request, len(notifierRefs))

	for i, notifierRef := range notifierRefs {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      notifierRef.Name,
				Namespace: provider.GetNamespace(),
			},
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *NotifierReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ddnsv1alpha1.Notifier{}).
		Watches(
			&ddnsv1alpha1.Provider{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForProvider),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r NotifierReconciler) updateObservedGeneration(notifiers *ddnsv1alpha1.Notifier, observedGeneration int64) func() bool {
	return func() bool {
		if notifiers.Status.ObservedGeneration == observedGeneration {
			return false
		}

		notifiers.Status.ObservedGeneration = observedGeneration

		return true
	}
}

func (r NotifierReconciler) updateIsReady(notifiers *ddnsv1alpha1.Notifier, isReady bool) func() bool {
	return func() bool {
		if notifiers.Status.IsReady == isReady {
			return false
		}

		notifiers.Status.IsReady = isReady

		return true
	}
}
