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
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
)

var _ = Describe("Provider Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()
		dummyIp := "127.0.0.1"
		dummyProviderIP := "127.0.0.2"
		var controllerReconciler *ProviderReconciler

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
			} else {
				Expect(err).NotTo(HaveOccurred())
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
			} else {
				Expect(err).NotTo(HaveOccurred())
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
						SecretName:    secretNamespacedName.Name,
						ConfigMap:     configMapNamespacedName.Name,
						RetryInterval: 123,
						NotifierRefs:  []ddnsv1alpha1.ResourceRef{},
					},
					Status: ddnsv1alpha1.ProviderStatus{},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the ProviderReconciler")
			controllerReconciler = &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{}, nil
				},
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

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully requeue the reqeust for an interval equal to the spec", func() {
			By("Reconciling the created resource")

			provider := &ddnsv1alpha1.Provider{}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
			Expect(result.RequeueAfter).To(Equal(time.Second * 123))
		})

		It("should set correct conditions", func() {
			By("Reconciling the created resource")

			provider := &ddnsv1alpha1.Provider{}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.ObservedGeneration).To(Equal(int64(1)))
			Expect(provider.Status.Conditions).To(HaveLen(3))
			Expect(meta.IsStatusConditionTrue(provider.Status.Conditions, "ConfigMap")).To(BeTrue())
			Expect(meta.IsStatusConditionTrue(provider.Status.Conditions, "Secret")).To(BeTrue())
			Expect(meta.IsStatusConditionTrue(provider.Status.Conditions, "Client")).To(BeTrue())

			secretCondition := meta.FindStatusCondition(provider.Status.Conditions, "Secret")
			Expect(secretCondition.Message).To(Equal(fmt.Sprintf("Secret %s found", secretNamespacedName.Name)))

			configMapCondition := meta.FindStatusCondition(provider.Status.Conditions, "ConfigMap")
			Expect(configMapCondition.Message).To(Equal(fmt.Sprintf("ConfigMap %s found", configMapNamespacedName.Name)))

			clientCondition := meta.FindStatusCondition(provider.Status.Conditions, "Client")
			Expect(clientCondition.Message).To(Equal("Client created"))
		})

		It("should set correct IPs if ProviderIP is empty", func() {
			By("Reconciling the created resource")

			provider := &ddnsv1alpha1.Provider{}

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: "",
					}, nil
				},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(dummyIp))
			Expect(provider.Status.ProviderIP).To(Equal(dummyIp))
		})

		It("should set correct IPs if ProviderIP is different", func() {
			By("Reconciling the created resource")

			calledCounter := 0
			provider := &ddnsv1alpha1.Provider{}

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: dummyProviderIP,
						SetIPInterceptor: func(ip string) {
							calledCounter++

							Expect(calledCounter).To(Equal(1))
							Expect(ip).To(Equal(dummyIp))
						},
					}, nil
				},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(dummyIp))
			Expect(provider.Status.ProviderIP).To(Equal(dummyIp))
		})

		It("should set correct IPs if called multiple times", func() {
			By("Reconciling the created resource")

			calledCounter := 0
			provider := &ddnsv1alpha1.Provider{}

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: dummyProviderIP,
						SetIPInterceptor: func(ip string) {
							calledCounter++

							Expect(ip).To(Equal(dummyIp))
						},
					}, nil
				},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(dummyIp))
			Expect(provider.Status.ProviderIP).To(Equal(dummyIp))

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(dummyIp))
			Expect(provider.Status.ProviderIP).To(Equal(dummyIp))

			Expect(calledCounter).To(Equal(2))
		})

		It("should change ProviderIP if the PublicIP changes", func() {
			By("Reconciling the created resource")

			// Overwrite the providerNamespacedName to create a new resource
			providerNamespacedName := types.NamespacedName{
				Name:      "provider-with-existing-status",
				Namespace: "default",
			}

			calledCounter := 0
			provider := &ddnsv1alpha1.Provider{}

			By("creating the custom resource for the Kind Provider with existing status")
			err := k8sClient.Get(ctx, providerNamespacedName, provider)
			if err != nil && errors.IsNotFound(err) {
				resource := &ddnsv1alpha1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      providerNamespacedName.Name,
						Namespace: providerNamespacedName.Namespace,
					},
					Spec: ddnsv1alpha1.ProviderSpec{
						Name:          "Cloudflare",
						SecretName:    secretNamespacedName.Name,
						ConfigMap:     configMapNamespacedName.Name,
						RetryInterval: 900,
						NotifierRefs:  []ddnsv1alpha1.ResourceRef{},
					},
					// Currently pointing to one IP, but it will be changed in the test
					Status: ddnsv1alpha1.ProviderStatus{
						PublicIP:   dummyProviderIP,
						ProviderIP: dummyProviderIP,
					},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				// Cleanup the resource after the test
				defer func() {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, resource))).NotTo(HaveOccurred())
				}()
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: dummyProviderIP,
						SetIPInterceptor: func(ip string) {
							calledCounter++

							Expect(ip).To(Equal(dummyIp))
						},
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(dummyIp))
			Expect(provider.Status.ProviderIP).To(Equal(dummyIp))

			Expect(calledCounter).To(Equal(1))
		})

		It("should not reconcile with unexisting configMap", func() {
			By("Reconciling the created resource")

			// Overwrite the providerNamespacedName to create a new resource
			providerNamespacedName := types.NamespacedName{
				Name:      "provider-with-wrong-configmap",
				Namespace: "default",
			}

			provider := &ddnsv1alpha1.Provider{}

			err := k8sClient.Get(ctx, providerNamespacedName, provider)
			if err != nil && errors.IsNotFound(err) {
				By("creating the custom resource for the Kind Provider with unexisting secret")
				resource := &ddnsv1alpha1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      providerNamespacedName.Name,
						Namespace: providerNamespacedName.Namespace,
					},
					Spec: ddnsv1alpha1.ProviderSpec{
						Name:          "Cloudflare",
						SecretName:    secretNamespacedName.Name,
						ConfigMap:     "unexisting-configmap",
						RetryInterval: 900,
						NotifierRefs:  []ddnsv1alpha1.ResourceRef{},
					},
					// Currently pointing to one IP, but it will be changed in the test
					Status: ddnsv1alpha1.ProviderStatus{},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				// Cleanup the resource after the test
				defer func() {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, resource))).NotTo(HaveOccurred())
				}()
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.Conditions).To(HaveLen(2))
			Expect(meta.IsStatusConditionFalse(provider.Status.Conditions, "ConfigMap")).To(BeTrue())

			condition := meta.FindStatusCondition(provider.Status.Conditions, "ConfigMap")
			Expect(condition.Message).To(Equal("ConfigMap unexisting-configmap not found"))
		})

		It("should not reconcile with unexisting secret", func() {
			By("Reconciling the created resource")

			// Overwrite the providerNamespacedName to create a new resource
			providerNamespacedName := types.NamespacedName{
				Name:      "provider-with-wrong-secret",
				Namespace: "default",
			}

			provider := &ddnsv1alpha1.Provider{}

			err := k8sClient.Get(ctx, providerNamespacedName, provider)
			if err != nil && errors.IsNotFound(err) {
				By("creating the custom resource for the Kind Provider with unexisting secret")
				resource := &ddnsv1alpha1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      providerNamespacedName.Name,
						Namespace: providerNamespacedName.Namespace,
					},
					Spec: ddnsv1alpha1.ProviderSpec{
						Name:          "Cloudflare",
						SecretName:    "unexisting-secret",
						ConfigMap:     configMapNamespacedName.Name,
						RetryInterval: 900,
						NotifierRefs:  []ddnsv1alpha1.ResourceRef{},
					},
					// Currently pointing to one IP, but it will be changed in the test
					Status: ddnsv1alpha1.ProviderStatus{},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())

				// Cleanup the resource after the test
				defer func() {
					Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, resource))).NotTo(HaveOccurred())
				}()
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.Conditions).To(HaveLen(1))
			Expect(meta.IsStatusConditionFalse(provider.Status.Conditions, "Secret")).To(BeTrue())

			condition := meta.FindStatusCondition(provider.Status.Conditions, "Secret")
			Expect(condition.Message).To(Equal("Secret unexisting-secret not found"))
		})

		It("should not reconcile if cannot fetch public IP", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return "", fmt.Errorf("cannot fetch public IP")
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: "",
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot fetch public IP"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(""))
		})

		It("should not reconcile if the ClientFactory cannot create a provider", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return nil, fmt.Errorf("cannot create client")
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot create client"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.Conditions).To(HaveLen(3))
			Expect(meta.IsStatusConditionFalse(provider.Status.Conditions, "Client")).To(BeTrue())

			condition := meta.FindStatusCondition(provider.Status.Conditions, "Client")
			Expect(condition.Message).To(Equal("could not create client: cannot create client"))
		})

		It("should not reconcile if the ProviderIP cannot be fetched", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP:         "",
						GetIPError: fmt.Errorf("cannot get IP"),
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot get IP"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())

			Expect(provider.Status.PublicIP).To(Equal(dummyIp))
			Expect(provider.Status.ProviderIP).To(Equal(""))
		})

		It("should not reconcile if the ProviderIP cannot be set", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP:         "",
						SetIPError: fmt.Errorf("cannot set IP"),
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot set IP"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not reconcile if we cannot patch the public ip in the Status", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			clientWrapper := &ClientWrapper{
				Client:           k8sClient,
				PatchStatusError: fmt.Errorf("cannot patch status"),
			}

			controllerReconciler := &ProviderReconciler{
				Client: clientWrapper,
				Scheme: clientWrapper.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: "",
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot patch status"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not reconcile if we cannot patch the provider ip in the Status", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			clientWrapper := &ClientWrapper{
				Client:           k8sClient,
				PatchStatusError: fmt.Errorf("cannot patch status"),
				PatchStatusIndex: 4,
			}

			controllerReconciler := &ProviderReconciler{
				Client: clientWrapper,
				Scheme: clientWrapper.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: dummyIp,
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot patch status"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not reconcile if we cannot patch the provider ip status after setting the ip in the provider", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			clientWrapper := &ClientWrapper{
				Client:           k8sClient,
				PatchStatusError: fmt.Errorf("cannot patch status"),
				PatchStatusIndex: 5,
			}

			controllerReconciler := &ProviderReconciler{
				Client: clientWrapper,
				Scheme: clientWrapper.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: "1.1.1.1",
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot patch status"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not reconcile if we cannot patch the observed generation", func() {
			provider := &ddnsv1alpha1.Provider{}
			var err error

			clientWrapper := &ClientWrapper{
				Client:           k8sClient,
				PatchStatusError: fmt.Errorf("cannot patch status"),
				PatchStatusIndex: 6,
			}

			controllerReconciler := &ProviderReconciler{
				Client: clientWrapper,
				Scheme: clientWrapper.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{
						IP: "1.1.1.1",
					}, nil
				},
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: providerNamespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cannot patch status"))

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore provider not found errors", func() {
			unexistingNamespacedName := types.NamespacedName{
				Name:      "unexisting-provider",
				Namespace: "default",
			}
			provider := &ddnsv1alpha1.Provider{}
			var err error

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: unexistingNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, unexistingNamespacedName, provider)
			Expect(err).To(HaveOccurred())
		})

		It("should return a new manager when SetupWithManager is called", func() {
			mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{})

			Expect(err).NotTo(HaveOccurred())

			controllerReconciler := &ProviderReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				IPProvider: func() (string, error) {
					return dummyIp, nil
				},
				ClientFactory: func(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (clients.Client, error) {
					return MockClient{}, nil
				},
			}

			err = controllerReconciler.SetupWithManager(mgr)

			Expect(err).NotTo(HaveOccurred())
		})
	})
})
