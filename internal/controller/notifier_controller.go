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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	notifierConditions "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/notifier/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/notifiers"
	"github.com/go-logr/logr"
)

// NotifierReconciler reconciles a Notifier object
type NotifierReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=notifiers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=notifiers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=notifiers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Notifier object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *NotifierReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	notifier := &ddnsv1alpha1.Notifier{}

	if err := r.Get(ctx, req.NamespacedName, notifier); err != nil {
		log.Error(err, "unable to fetch Notifier")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Notifier fetched", "notifier", notifier.Spec)

	notifierClient, err := r.FetchNotifier(ctx, req, notifier, log)
	if err != nil {
		log.Error(err, "unable to fetch notifier")
		return ctrl.Result{}, err
	}

	if notifier.Status.ObservedGeneration != notifier.GetGeneration() {
		if err = notifierClient.SendGreetings(); err != nil {
			log.Error(err, "unable to send greetings")

			condition := metav1.Condition{
				Type:    notifierConditions.ClientConditionType,
				Reason:  notifierConditions.ClientAuthFailed,
				Message: fmt.Sprintf("unable to send greetings: %s", err),
				Status:  metav1.ConditionFalse,
			}

			r.UpdateConditions(ctx, notifier, condition, log)
			return ctrl.Result{}, err
		}

		notifier.Status.IsReady = true
		notifier.Status.ObservedGeneration = notifier.GetGeneration()

		if err = r.Status().Update(ctx, notifier); err != nil {
			log.Error(err, "unable to update Notifier status")
			return ctrl.Result{}, err
		}
	}

	providers := &v1alpha1.ProviderList{}
	if err := r.List(ctx, providers); err != nil {
		log.Error(err, "unable to list Providers")
		return ctrl.Result{}, err
	}

	for _, provider := range providers.Items {
		for _, notifiers := range provider.Spec.NotifierRefs {
			namespace := notifiers.Namespace
			if notifiers.Name == notifier.Name && (namespace == notifier.Namespace || namespace == "") {
				log.Info("Provider is referencing notifier", "provider", provider.Name)
				// Check if annotation with the notifier namespacedName is present
				if provider.Annotations != nil {
					annotation := fmt.Sprintf("%s_%s", req.Name, req.Namespace)
					if value, ok := provider.Annotations[annotation]; ok {
						if value == provider.Status.PublicIP {
							log.Info("Public IP is the same as the last one, not notifying", "publicIP", value)
							continue
						} else {
							log.Info("Public IP is different, notifying", "publicIP", value)

							message := fmt.Sprintf("Public IP changed to %s, old was %s", provider.Status.PublicIP, value)

							if err = notifierClient.SendNotification(message); err != nil {
								log.Error(err, "unable to send notification")
								return ctrl.Result{}, err
							}

							provider.Annotations[annotation] = provider.Status.PublicIP
							if err = r.Update(ctx, &provider); err != nil {
								log.Error(err, "unable to update Provider")
								return ctrl.Result{}, err
							}
						}
					}
				}
			}
		}
	}

	return ctrl.Result{}, nil
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
		log.Error(err, "unable to fetch ConfigMap")
		return nil, err
	}

	secret, err := r.FetchSecret(ctx, req, notifier, log)
	if err != nil {
		log.Error(err, "unable to fetch Secret")
		return nil, err
	}

	notifierClient, err := r.CreateNotifier(ctx, notifier, secret, configMap)
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

	r.UpdateConditions(ctx, notifier, condition, log)

	return notifierClient, err
}

func (r *NotifierReconciler) CreateNotifier(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	secret *corev1.Secret,
	configMap *corev1.ConfigMap,
) (notifiers.Notifier, error) {
	switch notifier.Spec.Name {
	case notifiers.Webhook:
		if secret.Data["url"] == nil {
			return nil, fmt.Errorf("`url` not found in secret")
		}

		return &notifiers.WebhookNotifier{
			Url: string(secret.Data["url"]),
		}, nil
	default:
		return nil, fmt.Errorf("unknown notifier %s", notifier.Spec.Name)
	}
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

	r.UpdateConditions(ctx, notifier, condition, log)

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

	r.UpdateConditions(ctx, notifier, condition, log)

	return secret, nil
}

// UpdateConditions updates the conditions of the Notifier
// Sets the ObservedGeneration of the condition to the generation of the Notifier
func (r *NotifierReconciler) UpdateConditions(
	ctx context.Context,
	notifier *ddnsv1alpha1.Notifier,
	condition metav1.Condition,
	log logr.Logger,
) error {
	condition.ObservedGeneration = notifier.GetGeneration()
	changed := meta.SetStatusCondition(&notifier.Status.Conditions, condition)

	if !changed {
		return nil
	}

	if err := r.Status().Update(ctx, notifier); err != nil {
		log.Error(err, "unable to update Notifier status")
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NotifierReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ddnsv1alpha1.Notifier{}).
		Watches(
			&v1alpha1.Provider{},
			&handler.EnqueueRequestForObject{},
		).
		Complete(r)
}
