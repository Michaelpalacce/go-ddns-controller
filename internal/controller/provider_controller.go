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

	log.Info("Reconciling Provider")

	provider := &ddnsv1alpha1.Provider{}
	if err := r.Get(ctx, req.NamespacedName, provider); err != nil {
		log.Error(err, "unable to fetch Provider, skipping")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Provider fetched", "provider", provider.Spec)

	publicIp, err := r.FetchPublicIp(ctx, req, provider, log)
	if err != nil {
		log.Error(err, "unable to fetch public IP")
		return ctrl.Result{}, err
	}

	if provider.Status.PublicIP != publicIp {
		log.Info("Public IP changed", "old", provider.Status.PublicIP, "new", publicIp)
	}

	provider.Status.PublicIP = publicIp

	providerClient, err := r.FetchClient(ctx, req, provider, log)
	if err != nil {
		log.Error(err, "unable to fetch client")
		return ctrl.Result{}, err
	}

	providerIp, err := providerClient.GetIp()
	if err != nil {
		log.Error(err, "trying to get IP from provider failed, maybe auth error?")

		return ctrl.Result{}, err
	}

	if provider.Status.PublicIP != providerIp {
		log.Info("IP Missmatch", "publicIP", provider.Status.PublicIP, "providerIP", providerIp)
		providerClient.SetIp(provider.Status.PublicIP)
	}

	provider.Status.ProviderIP = provider.Status.PublicIP

	if err := r.Status().Update(ctx, provider); err != nil {
		log.Error(err, "unable to update Provider status")
		return ctrl.Result{}, err
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

// FetchPublicIp will fetch the public IP of the machine that is running goip
// it will also update the status of the Provider so logic is isolated in this function
func (r *ProviderReconciler) FetchPublicIp(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
	log logr.Logger,
) (string, error) {
	var (
		message string
		status  metav1.ConditionStatus
	)

	ip, err := network.GetPublicIp()

	if err != nil {
		message = fmt.Sprintf("error while trying to fetch public IP: %s", err)
		status = metav1.ConditionFalse
	} else {
		message = fmt.Sprintf("Public IP fetched: %s", ip)
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               providerConditions.PublicIpConditionType,
		Reason:             providerConditions.PublicIpFetched,
		Message:            message,
		Status:             status,
		ObservedGeneration: provider.GetGeneration(),
	}

	meta.SetStatusCondition(&provider.Status.Conditions, condition)

	if statusErr := r.Status().Update(ctx, provider); statusErr != nil {
		log.Error(statusErr, "unable to update Provider status", "condition", condition)
	}

	return ip, err
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
	var (
		err     error
		message string
		status  metav1.ConditionStatus
	)

	secret, err := r.FetchSecret(ctx, req, provider, log)
	if err != nil {
		return nil, err
	}

	log.Info("Secret fetched")

	configMap, err := r.FetchConfig(ctx, req, provider, log)
	if err != nil {
		return nil, err
	}

	log.Info("ConfigMap fetched", "configMap", configMap)

	providerClient, err := r.CreateClient(provider.Spec.Name, secret, configMap, log)
	if err != nil {
		message = fmt.Sprintf("could not create client: %s", err)
		status = metav1.ConditionFalse
	} else {
		message = "Client created"
		status = metav1.ConditionTrue
	}

	condition := metav1.Condition{
		Type:               providerConditions.ClientCreatedConditionType,
		Reason:             providerConditions.ClientCreated,
		ObservedGeneration: provider.GetGeneration(),
		Message:            message,
		Status:             status,
	}

	meta.SetStatusCondition(&provider.Status.Conditions, condition)

	if statusErr := r.Status().Update(ctx, provider); statusErr != nil {
		log.Error(statusErr, "unable to update Provider status", "condition", condition)
	}

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
