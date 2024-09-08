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
	"github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/notifiers"
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
	notifier := &ddnsv1alpha1.Notifier{}

	if err := r.Get(ctx, req.NamespacedName, notifier); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	notifier.Conditions().FillConditions()

	notifierClient, err := r.fetchNotifier(ctx, req, notifier)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to fetch notifier: %w", err)
	}

	if !notifier.Status.IsReady {
		if err := r.markAsReady(ctx, notifier, notifierClient); err != nil {
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
		if err = r.notifyOfChange(ctx, req, &provider, notifier, notifierClient); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to notify of change: %w", err)
		}
	}

	if err := r.patchStatus(ctx, notifier, r.patchObservedGeneration(notifier.GetGeneration())); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to update Notifier status: %w", err)
	}

	return ctrl.Result{}, nil
}

// ============================================== PRIVATE FUNCTIONS ==============================================

// markAsReady marks the Notifier as ready
// Ready means that the Notifier has been successfully created and a greeting message has been sent
func (r *NotifierReconciler) markAsReady(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	notifierClient notifiers.Notifier,
) (err error) {
	condOptions := []conditions.ConditionOption{}

	if err = notifierClient.SendGreetings(notifier); err != nil {
		message := fmt.Sprintf("unable to send greetings: %s", err)
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ClientCommunication", message),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ClientCommunication", "Communications established"),
			conditions.True(),
		)
	}

	conditions.PatchConditions(ctx, r.Client, notifier, ddnsv1alpha1.NotifierConditionTypeClient, condOptions...)

	if err != nil {
		return fmt.Errorf("unable to send greetings: %w", err)
	}

	if err := r.patchStatus(ctx, notifier, r.patchIsReady(true)); err != nil {
		return fmt.Errorf("unable to mark Notifier as ready: %w", err)
	}

	return nil
}

// notifyOfChange sends a notification to the notifierClient
// We need to first update the annotation of the Provider with the new IP, then send the notification
// this is done to avoid issues with the resouceVersion of the Provider object
func (r *NotifierReconciler) notifyOfChange(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	notifier *ddnsv1alpha1.Notifier,
	notifierClient notifiers.Notifier,
) error {
	log := log.FromContext(ctx)
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
	if provider.Annotations == nil {
		provider.Annotations = make(map[string]string)
	}
	provider.Annotations[annotation] = provider.Status.ProviderIP

	if err := notifierClient.SendNotification(message); err != nil {
		log.Error(err, "unable to send notification")

		if err := r.patchStatus(ctx, notifier, r.patchIsReady(false)); err != nil {
			log.Error(err, "unable to mark Notifier as not ready")
		}

		condOptions := []conditions.ConditionOption{
			conditions.WithReasonAndMessage("ClientCommunication", fmt.Sprintf("unable to send notification: %s", err)),
			conditions.False(),
		}

		conditions.PatchConditions(ctx, r.Client, notifier, ddnsv1alpha1.NotifierConditionTypeClient, condOptions...)

		return err
	}

	condOptions := []conditions.ConditionOption{
		conditions.WithReasonAndMessage("ClientCommunication", "Notification sent"),
		conditions.True(),
	}

	conditions.PatchConditions(ctx, r.Client, notifier, ddnsv1alpha1.NotifierConditionTypeClient, condOptions...)

	if err := r.Patch(ctx, provider, patch); err != nil {
		return err
	}

	return nil
}

func (r *NotifierReconciler) fetchNotifier(
	ctx context.Context,
	req ctrl.Request,
	notifier *ddnsv1alpha1.Notifier,
) (notifiers.Notifier, error) {
	var err error

	configMap, err := r.fetchConfig(ctx, req, notifier)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch ConfigMap: %w", err)
	}

	secret, err := r.fetchSecret(ctx, req, notifier)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch Secret: %w", err)
	}

	condOptions := []conditions.ConditionOption{}

	notifierClient, err := r.NotifierFactory(notifier, secret, configMap)
	if err != nil {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ClientCreated", fmt.Sprintf("could not create client: %s", err)),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ClientCreated", "Client created"),
			conditions.True(),
		)
	}

	conditions.PatchConditions(ctx, r.Client, notifier, ddnsv1alpha1.NotifierConditionTypeClient, condOptions...)

	return notifierClient, err
}

func (r *NotifierReconciler) fetchConfig(
	ctx context.Context,
	req ctrl.Request,
	notifier *ddnsv1alpha1.Notifier,
) (*corev1.ConfigMap, error) {
	var (
		configMap *corev1.ConfigMap
		err       error
	)

	condOptions := []conditions.ConditionOption{}

	configMap = &corev1.ConfigMap{}
	if err = r.Get(ctx, types.NamespacedName{Name: notifier.Spec.ConfigMap, Namespace: req.Namespace}, configMap); err != nil {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ConfigMapFound", err.Error()),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ConfigMapFound", fmt.Sprintf("ConfigMap %s found", notifier.Spec.ConfigMap)),
			conditions.True(),
		)
	}

	conditions.PatchConditions(ctx, r.Client, notifier, ddnsv1alpha1.NotifierConditionTypeConfigMap, condOptions...)

	return configMap, err
}

func (r *NotifierReconciler) fetchSecret(
	ctx context.Context,
	req ctrl.Request,
	notifier *ddnsv1alpha1.Notifier,
) (*corev1.Secret, error) {
	var (
		err    error
		secret *corev1.Secret
	)

	condOptions := []conditions.ConditionOption{}

	secret = &corev1.Secret{}
	if err = r.Get(ctx, types.NamespacedName{Name: notifier.Spec.SecretName, Namespace: req.Namespace}, secret); err != nil {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("SecretFound", err.Error()),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("SecretFound", fmt.Sprintf("Secret %s found", notifier.Spec.SecretName)),
			conditions.True(),
		)
	}

	conditions.PatchConditions(ctx, r.Client, notifier, ddnsv1alpha1.NotifierConditionTypeSecret, condOptions...)

	return secret, nil
}

func (r *NotifierReconciler) patchStatus(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	apply func(notifier *ddnsv1alpha1.Notifier) bool,
) error {
	patch := client.MergeFrom(notifier.DeepCopy())
	if apply(notifier) {
		if err := r.Status().Patch(ctx, notifier, patch); err != nil {
			return err
		}
	}

	return nil
}

// ============================================ SETUP FUNCTIONS ============================================

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

// ============================================ PATCH FUNCTIONS ============================================

func (r NotifierReconciler) patchObservedGeneration(observedGeneration int64) func(notifiers *ddnsv1alpha1.Notifier) bool {
	return func(notifiers *ddnsv1alpha1.Notifier) bool {
		if notifiers.Status.ObservedGeneration == observedGeneration {
			return false
		}

		notifiers.Status.ObservedGeneration = observedGeneration

		return true
	}
}

func (r NotifierReconciler) patchIsReady(isReady bool) func(notifiers *ddnsv1alpha1.Notifier) bool {
	return func(notifiers *ddnsv1alpha1.Notifier) bool {
		if notifiers.Status.IsReady == isReady {
			return false
		}

		notifiers.Status.IsReady = isReady

		return true
	}
}
