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
	"encoding/base64"
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
	"sigs.k8s.io/controller-runtime/pkg/log"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	providerConditions "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/provider/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
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
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Provider")

	provider := &ddnsv1alpha1.Provider{}
	if err := r.Get(ctx, req.NamespacedName, provider); err != nil {
		log.Error(err, "unable to fetch Provider, skipping")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Provider fetched", "provider", provider.Spec)

	providerClient, err := r.FetchClient(ctx, req, provider, log)

	if err != nil {
		log.Error(err, "unable to create client")

		condition := metav1.Condition{
			Type:               providerConditions.ClientCreatedConditionType,
			Reason:             providerConditions.ClientCreated,
			ObservedGeneration: provider.GetGeneration(),
			Message:            err.Error(),
			Status:             metav1.ConditionFalse,
		}

		meta.SetStatusCondition(&provider.Status.Conditions, condition)

		if statusErr := r.Status().Update(ctx, provider); statusErr != nil {
			log.Error(statusErr, "unable to update Provider status", "condition", condition)
		}

		return ctrl.Result{}, err
	} else {
		log.Info("Client fetched", "client", providerClient)

		condition := metav1.Condition{
			Type:               providerConditions.ClientCreatedConditionType,
			Reason:             providerConditions.ClientCreated,
			ObservedGeneration: provider.GetGeneration(),
			Message:            "Client created",
			Status:             metav1.ConditionTrue,
		}

		meta.SetStatusCondition(&provider.Status.Conditions, condition)

		if statusErr := r.Status().Update(ctx, provider); statusErr != nil {
			log.Error(statusErr, "unable to update Provider status", "condition", condition)
		}
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Minute * 15,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ddnsv1alpha1.Provider{}).
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
		fmt.Println("Error: ", err)

		message = fmt.Sprintf("Secret %s not found", provider.Spec.SecretName)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("Secret %s found", provider.Spec.SecretName)
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               providerConditions.SecretConditionType,
		Reason:             providerConditions.SecretFound,
		ObservedGeneration: provider.GetGeneration(),
		Message:            message,
		Status:             status,
	}

	meta.SetStatusCondition(&provider.Status.Conditions, condition)

	if statusErr := r.Status().Update(ctx, provider); statusErr != nil {
		log.Error(statusErr, "unable to update Provider status", "condition", condition)
	}

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
		Type:               providerConditions.ConfigMapConditionType,
		Reason:             providerConditions.ConfigMapFound,
		ObservedGeneration: provider.GetGeneration(),
		Message:            message,
		Status:             status,
	}

	meta.SetStatusCondition(&provider.Status.Conditions, condition)

	if statusErr := r.Status().Update(ctx, provider); statusErr != nil {
		log.Error(statusErr, "unable to update Provider status", "condition", condition)
	}

	return configMap, err
}

func (r *ProviderReconciler) FetchClient(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	log logr.Logger,
) (clients.Client, error) {
	secret, err := r.FetchSecret(ctx, req, provider, log)
	if err != nil {
		return nil, err
	}

	configMap, err := r.FetchConfig(ctx, req, provider, log)
	if err != nil {
		return nil, err
	}

	client, err := r.CreateClient(provider.Spec.Name, secret, configMap, log)
	if err != nil {
		return nil, err
	}

	return client, nil
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

		configMap := configMap.Data["config"]

		err := json.Unmarshal([]byte(configMap), &cloudflareConfig)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal the config: %s", err)
		}

		if secret.Data["apiToken"] == nil {
			return nil, fmt.Errorf("`apiToken` not found in secret")
		}

		data, err := base64.StdEncoding.DecodeString(string(secret.Data["apiToken"]))
		if err != nil {
			return nil, fmt.Errorf("could not decode the apiToken: %s", err)
		}

		client, err = clients.NewCloudflareClient(cloudflareConfig, string(data), log)
		if err != nil {
			return nil, fmt.Errorf("could not create a Cloudflare client: %s", err)
		}
	default:
		return nil, fmt.Errorf("could not create a provider of type: %s", name)
	}

	return client, nil
}
