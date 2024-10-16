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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	"github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
)

type (
	IPProvider    func(string) (string, error)
	ClientFactory func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error)
)

// ProviderReconciler reconciles a Provider object
type ProviderReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	IPProvider    IPProvider
	ClientFactory ClientFactory
}

// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ddns.stefangenov.site,resources=providers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile will reconcile the Provider object
func (r *ProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err            error
		providerClient clients.Client
		providerIps    []string
		publicIp       string
	)

	provider := &ddnsv1alpha1.Provider{}
	if err = r.Get(ctx, req.NamespacedName, provider); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if publicIp, err = r.IPProvider(provider.Spec.CustomIPProvider); err != nil {
		return ctrl.Result{}, err
	}

	provider.Conditions().FillConditions()

	if err = r.patchStatus(ctx, provider, r.patchPublicIp(publicIp)); err != nil {
		return ctrl.Result{}, err
	}

	if providerClient, err = r.fetchClient(ctx, req, provider); err != nil {
		return ctrl.Result{}, err
	}

	if providerIps, err = providerClient.GetIp(); err != nil {
		return ctrl.Result{}, err
	}

	// Remove duplicates
	uniqueIps := r.uniqueIps(providerIps)

	if err := r.patchStatus(ctx, provider, r.patchProviderIp(strings.Join(uniqueIps, ", "))); err != nil {
		return ctrl.Result{}, err
	}

	if provider.Status.PublicIP != provider.Status.ProviderIP {
		log.FromContext(ctx).Info("IPs desynced, updating provider IP")

		if err := providerClient.SetIp(provider.Status.PublicIP); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.patchStatus(ctx, provider, r.patchProviderIp(provider.Status.PublicIP)); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.patchStatus(ctx, provider, r.patchObservedGeneration()); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: time.Second * time.Duration(provider.Spec.RetryInterval),
	}, nil
}

// =================================================== PRIVATE FUNCTIONS ===================================================

// uniqueIps will remove duplicates from a list of IPs
func (r *ProviderReconciler) uniqueIps(ips []string) []string {
	uniqueIps := []string{}
	ipMap := make(map[string]bool)

	for _, ip := range ips {
		if !ipMap[ip] {
			ipMap[ip] = true
			uniqueIps = append(uniqueIps, ip)
		}
	}

	return uniqueIps
}

// fetchSecret will fetch the secret from the namespace and set the status of the Provider
// it will also update the status of the Provider so logic is isolated in this function
func (r *ProviderReconciler) fetchSecret(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
) (*corev1.Secret, error) {
	var (
		err    error
		secret *corev1.Secret
	)

	condOptions := []conditions.ConditionOption{}

	secret = &corev1.Secret{}
	if err = r.Get(ctx, types.NamespacedName{Name: provider.Spec.SecretName, Namespace: req.Namespace}, secret); err != nil {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("SecretFound", err.Error()),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("SecretFound", fmt.Sprintf("Secret %s found", provider.Spec.SecretName)),
			conditions.True(),
		)
	}

	_ = conditions.PatchConditions(ctx, r.Client, provider, ddnsv1alpha1.ProviderConditionTypeSecret, condOptions...)

	return secret, err
}

func (r *ProviderReconciler) fetchConfig(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
) (*corev1.ConfigMap, error) {
	var (
		configMap *corev1.ConfigMap
		err       error
	)

	condOptions := []conditions.ConditionOption{}

	configMap = &corev1.ConfigMap{}
	if err = r.Get(ctx, types.NamespacedName{Name: provider.Spec.ConfigMap, Namespace: req.Namespace}, configMap); err != nil {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ConfigMapFound", err.Error()),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ConfigMapFound", fmt.Sprintf("ConfigMap %s found", provider.Spec.ConfigMap)),
			conditions.True(),
		)
	}

	_ = conditions.PatchConditions(ctx, r.Client, provider, ddnsv1alpha1.ProviderConditionTypeConfigMap, condOptions...)

	return configMap, err
}

func (r *ProviderReconciler) fetchClient(
	ctx context.Context,
	req ctrl.Request,
	provider *ddnsv1alpha1.Provider,
) (clients.Client, error) {
	secret, err := r.fetchSecret(ctx, req, provider)
	if err != nil {
		return nil, err
	}

	configMap, err := r.fetchConfig(ctx, req, provider)
	if err != nil {
		return nil, err
	}

	condOptions := []conditions.ConditionOption{}

	providerClient, err := r.ClientFactory(provider.Spec.Name, secret, configMap, log.FromContext(ctx))
	if err != nil {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ClientCreated", err.Error()),
			conditions.False(),
		)
	} else {
		condOptions = append(condOptions,
			conditions.WithReasonAndMessage("ClientCreated", "Client created successfully"),
			conditions.True(),
		)
	}

	_ = conditions.PatchConditions(ctx, r.Client, provider, ddnsv1alpha1.ProviderConditionTypeClient, condOptions...)

	return providerClient, err
}

func (r *ProviderReconciler) patchStatus(
	ctx context.Context,
	provider *ddnsv1alpha1.Provider,
	apply func(*ddnsv1alpha1.Provider) bool,
) error {
	patch := client.MergeFrom(provider.DeepCopy())
	if apply(provider) {
		if err := r.Status().Patch(ctx, provider, patch); err != nil {
			return err
		}
	}

	return nil
}

// =================================================== SETUP FUNCTIONS ===================================================

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

// =================================================== PATCH FUNCTIONS ===================================================

func (p ProviderReconciler) patchProviderIp(providerIp string) func(provider *ddnsv1alpha1.Provider) bool {
	return func(provider *ddnsv1alpha1.Provider) bool {
		if provider.Status.ProviderIP == providerIp {
			return false
		}

		provider.Status.ProviderIP = providerIp

		return true
	}
}

func (p ProviderReconciler) patchPublicIp(publicIp string) func(provider *ddnsv1alpha1.Provider) bool {
	return func(provider *ddnsv1alpha1.Provider) bool {
		if provider.Status.PublicIP == publicIp {
			return false
		}

		provider.Status.PublicIP = publicIp

		return true
	}
}

func (p ProviderReconciler) patchObservedGeneration() func(provider *ddnsv1alpha1.Provider) bool {
	return func(provider *ddnsv1alpha1.Provider) bool {
		if provider.Status.ObservedGeneration == provider.GetGeneration() {
			return false
		}
		provider.Status.ObservedGeneration = provider.GetGeneration()
		return true
	}
}
