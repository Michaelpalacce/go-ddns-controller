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
	"sigs.k8s.io/controller-runtime/pkg/log"

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
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.4/pkg/reconcile
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

	if providerClient, err = r.FetchClient(ctx, req, provider, log); err != nil {
		log.Error(err, "unable to fetch client")
		return ctrl.Result{}, err
	}

	if providerIp, err = providerClient.GetIp(); err != nil {
		log.Error(err, "trying to get IP from provider failed, maybe auth error?")
		return ctrl.Result{}, err
	}

	if publicIp, err = network.GetPublicIp(); err != nil {
		log.Error(err, "unable to fetch public IP")
		return ctrl.Result{}, err
	}

	if provider.Status.ProviderIP != providerIp {
		provider.Status.ProviderIP = providerIp
	}

	if provider.Status.PublicIP != publicIp {
		provider.Status.PublicIP = publicIp
	}

	if provider.Status.PublicIP != provider.Status.ProviderIP {
		if err := providerClient.SetIp(provider.Status.ProviderIP); err != nil {
			log.Error(err, "unable to set IP on provider")
			return ctrl.Result{}, err
		}

		provider.Status.ProviderIP = provider.Status.PublicIP

		if err := r.UpdateStatus(ctx, provider, log); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Second * 15,
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
		message = fmt.Sprintf("Secret %s not found", provider.Spec.SecretName)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("Secret %s found", provider.Spec.SecretName)
		status = metav1.ConditionTrue
	}

	condition := &metav1.Condition{
		Type:    providerConditions.SecretConditionType,
		Reason:  providerConditions.SecretFound,
		Message: message,
		Status:  status,
	}

	r.UpdateConditions(ctx, provider, condition, log)

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

	condition := &metav1.Condition{
		Type:    providerConditions.ConfigMapConditionType,
		Reason:  providerConditions.ConfigMapFound,
		Message: message,
		Status:  status,
	}

	r.UpdateConditions(ctx, provider, condition, log)

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

	condition := &metav1.Condition{
		Type:    providerConditions.ClientConditionType,
		Reason:  providerConditions.ClientCreated,
		Message: message,
		Status:  status,
	}

	r.UpdateConditions(ctx, provider, condition, log)

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
	condition *metav1.Condition,
	log logr.Logger,
) error {
	condition.ObservedGeneration = provider.GetGeneration()
	change := meta.SetStatusCondition(&provider.Status.Conditions, *condition)
	if change {
		return r.UpdateStatus(ctx, provider, log)
	}

	return nil
}

// UpdateStatus updates the status of the Notifier
func (r *ProviderReconciler) UpdateStatus(
	ctx context.Context,
	provider *ddnsv1alpha1.Provider,
	log logr.Logger,
) error {
	if err := r.Status().Update(ctx, provider); err != nil {
		return err
	}

	return nil
}
