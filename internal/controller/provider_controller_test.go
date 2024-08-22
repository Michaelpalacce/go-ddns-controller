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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
)

var _ = Describe("Provider Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		providerNamespacedName := types.NamespacedName{
			Name:      "test-provider",
			Namespace: "default",
		}

		configMapNamespacedName := types.NamespacedName{
			Name:      "cloudflare-config",
			Namespace: "default",
		}

		secretNamespacedName := types.NamespacedName{
			Name:      "cloudflare-secret",
			Namespace: "default",
		}

		BeforeEach(func() {
			var err error
			By("creating the ConfigMap for the Provider")

			err = k8sClient.Get(ctx, configMapNamespacedName, &corev1.ConfigMap{})
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapNamespacedName.Name,
						Namespace: configMapNamespacedName.Namespace,
					},
					Data: map[string]string{
						"config": `{
                          "cloudflare": {
                              "zones": [
                                  {
                                      "name": "example.com",
                                      "records": [
                                          {
                                              "name": "example.com",
                                              "proxied": true
                                          }
                                      ]
                                  }
                              ]
                          }
                        }`,
					},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("creating the Secret for the Provider")
			err = k8sClient.Get(ctx, secretNamespacedName, &corev1.Secret{})
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNamespacedName.Name,
						Namespace: secretNamespacedName.Namespace,
					},
					StringData: map[string]string{
						"apiToken": "test-token",
					},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("creating the custom resource for the Kind Provider")
			err = k8sClient.Get(ctx, providerNamespacedName, &ddnsv1alpha1.Provider{})
			if err != nil && errors.IsNotFound(err) {
				resource := &ddnsv1alpha1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      providerNamespacedName.Name,
						Namespace: providerNamespacedName.Namespace,
					},
					Spec: ddnsv1alpha1.ProviderSpec{
						Name:          "Cloudflare",
						SecretName:    "cloudflare-secret",
						ConfigMap:     configMapNamespacedName.Name,
						RetryInterval: 900,
						NotifierRefs:  []ddnsv1alpha1.ResourceRef{},
					},
					Status: ddnsv1alpha1.ProviderStatus{},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			providerResource := &ddnsv1alpha1.Provider{}
			secretResource := &corev1.Secret{}
			configMapResource := &corev1.ConfigMap{}

			Expect(k8sClient.Get(ctx, providerNamespacedName, providerResource)).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, secretNamespacedName, secretResource)).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, configMapNamespacedName, configMapResource)).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Provider and related resources")
			Expect(k8sClient.Delete(ctx, providerResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, configMapResource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			provider := &ddnsv1alpha1.Provider{}
			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IpProvider: func() (string, error) {
					return "127.0.0.1", nil
				},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: providerNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)

			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal("127.0.0.1"))
		})
	})
})
