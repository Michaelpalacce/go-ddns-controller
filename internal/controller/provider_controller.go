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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	providerConditions "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/provider/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
	"github.com/Michaelpalacce/go-ddns-controller/internal/network"
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile will reconcile the Provider object
func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var (
		err            error
		providerClient clients.Client
		providerIp     string
		publicIp       string
	)

	provider := &ddnsv1alpha1.Provider{}
	if err = r.Get(ctx, req.NamespacedName, provider); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Provider triggered")

	if publicIp, err = network.GetPublicIp(); err != nil {
		log.Error(err, "unable to fetch public IP")
		return ctrl.Result{}, err
	}

	if err := r.PatchStatus(ctx, provider, r.updatePublicIp(provider, publicIp), log); err != nil {
		return ctrl.Result{}, err
	}

	if providerClient, err = r.FetchClient(ctx, req, provider, log); err != nil {
		log.Error(err, "unable to fetch client")
		return ctrl.Result{}, err
	}

	if providerIp, err = providerClient.GetIp(); err != nil {
		log.Error(err, "trying to get IP from provider failed, maybe auth error?")
		return ctrl.Result{}, err
	}

	if err := r.PatchStatus(ctx, provider, r.updateProviderIp(provider, providerIp), log); err != nil {
		return ctrl.Result{}, err
	}

	if provider.Status.PublicIP != provider.Status.ProviderIP {
		log.Info("IPs desynced, updating provider IP")

		if err := providerClient.SetIp(provider.Status.PublicIP); err != nil {
			log.Error(err, "unable to set IP on provider")
			return ctrl.Result{}, err
		}

		if err := r.PatchStatus(ctx, provider, r.updateProviderIp(provider, provider.Status.PublicIP), log); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.PatchStatus(ctx, provider, func() bool {
		if provider.Status.ObservedGeneration == provider.GetGeneration() {
			return false
		}
		provider.Status.ObservedGeneration = provider.GetGeneration()
		return true
	}, log); err != nil {
		log.Error(err, "unable to update Notifier status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Second * time.Duration(provider.Spec.RetryInterval),
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ddnsv1alpha1.Provider{}).
		// WithEventFilter will only trigger the reconcile function if the observed generation is different from the new generation
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				newGeneration := e.ObjectNew.GetGeneration()
				observedGeneration := e.ObjectNew.DeepCopyObject().(*ddnsv1alpha1.Provider).Status.ObservedGeneration

				return observedGeneration != newGeneration
			},
		}).
		Complete(r)
}

// FetchSecret will fetch the secret from the namespace and set the status of the Provider
// it will also update the status of the Provider so logic is isolated in this function
func (r *ProviderReconciler) FetchSecret(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	log logr.Logger,
) (*corev1.Secret, error) {
	var (
		err     error
		message string
		status  metav1.ConditionStatus
		secret  *corev1.Secret
	)

	secret = &corev1.Secret{}
	if err = r.Get(ctx, types.NamespacedName{Name: provider.Spec.SecretName, Namespace: req.Namespace}, secret); err != nil {
		message = fmt.Sprintf("Secret %s not found", provider.Spec.SecretName)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("Secret %s found", provider.Spec.SecretName)
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:    providerConditions.SecretConditionType,
		Reason:  providerConditions.SecretFound,
		Message: message,
		Status:  status,
	}

	_ = r.UpdateConditions(ctx, provider, condition, log)

	return secret, nil
}

func (r *ProviderReconciler) FetchConfig(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	log logr.Logger,
) (*corev1.ConfigMap, error) {
	var (
		configMap *corev1.ConfigMap
		err       error
		message   string
		status    metav1.ConditionStatus
	)
	configMap = &corev1.ConfigMap{}

	if err = r.Get(ctx, types.NamespacedName{Name: provider.Spec.ConfigMap, Namespace: req.Namespace}, configMap); err != nil {
		message = fmt.Sprintf("ConfigMap %s not found", provider.Spec.ConfigMap)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("ConfigMap %s found", provider.Spec.ConfigMap)
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:    providerConditions.ConfigMapConditionType,
		Reason:  providerConditions.ConfigMapFound,
		Message: message,
		Status:  status,
	}

	_ = r.UpdateConditions(ctx, provider, condition, log)

	return configMap, err
}

func (r *ProviderReconciler) FetchClient(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	log logr.Logger,
) (clients.Client, error) {
	var (
		err     error
		message string
		status  metav1.ConditionStatus
	)

	secret, err := r.FetchSecret(ctx, req, provider, log)
	if err != nil {
		return nil, err
	}

	configMap, err := r.FetchConfig(ctx, req, provider, log)
	if err != nil {
		return nil, err
	}

	providerClient, err := r.CreateClient(provider.Spec.Name, secret, configMap, log)
	if err != nil {
		message = fmt.Sprintf("could not create client: %s", err)
		status = metav1.ConditionFalse
	} else {
		message = "Client created"
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:    providerConditions.ClientConditionType,
		Reason:  providerConditions.ClientCreated,
		Message: message,
		Status:  status,
	}

	_ = r.UpdateConditions(ctx, provider, condition, log)

	return providerClient, err
}

// CreateClientBasedOnInput will return an authenticated, fully loaded client
func (r *ProviderReconciler) CreateClient(
	name string,
	secret *corev1.Secret,
	configMap *corev1.ConfigMap,
	log logr.Logger,
) (clients.Client, error) {
	var client clients.Client
	switch name {
	case clients.Cloudflare:
		var cloudflareConfig clients.CloudflareConfig

		if configMap.Data["config"] == "" {
			return nil, fmt.Errorf("`config` not found in configMap")
		}

		configMap := configMap.Data["config"]

		err := json.Unmarshal([]byte(configMap), &cloudflareConfig)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal the config: %s", err)
		}

		if secret.Data["apiToken"] == nil {
			return nil, fmt.Errorf("`apiToken` not found in secret")
		}

		client, err = clients.NewCloudflareClient(cloudflareConfig, string(secret.Data["apiToken"]), log)
		if err != nil {
			return nil, fmt.Errorf("could not create a Cloudflare client: %s", err)
		}
	default:
		return nil, fmt.Errorf("could not create a provider of type: %s", name)
	}

	return client, nil
}

func (r *ProviderReconciler) UpdateConditions(
	ctx context.Context,
	provider *ddnsv1alpha1.Provider,
	condition metav1.Condition,
	log logr.Logger,
) error {
	return r.PatchStatus(ctx, provider, func() bool {
		condition.ObservedGeneration = provider.GetGeneration()

		return meta.SetStatusCondition(&provider.Status.Conditions, condition)
	}, log)
}

func (r *ProviderReconciler) PatchStatus(
	ctx context.Context,
	provider *ddnsv1alpha1.Provider,
	apply func() bool,
	log logr.Logger,
) error {
	patch := client.MergeFrom(provider.DeepCopy())
	if apply() {
		if err := r.Status().Patch(ctx, provider, patch); err != nil {
			log.Error(err, "unable to patch status")
			return err
		}
	}

	return nil
}

func (p ProviderReconciler) updateProviderIp(provider *ddnsv1alpha1.Provider, providerIp string) func() bool {
	return func() bool {
		if provider.Status.ProviderIP == providerIp {
			return false
		}

		provider.Status.ProviderIP = providerIp

		return true
	}
}

func (p ProviderReconciler) updatePublicIp(provider *ddnsv1alpha1.Provider, publicIp string) func() bool {
	return func() bool {
		if provider.Status.PublicIP == publicIp {
			return false
		}

		provider.Status.PublicIP = publicIp

		return true
	}
}
