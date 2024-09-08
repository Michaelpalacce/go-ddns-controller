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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
	"github.com/Michaelpalacce/go-ddns-controller/internal/notifiers"
)

// Pending, need to be implemented
var _ = Describe("Notifier Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		dummyIp := "127.0.0.1"
		notifier := &ddnsv1alpha1.Notifier{}
		ctx := context.Background()
		var err error
		var controllerNotifierReconciler *NotifierReconciler
		var controllerReconciler *ProviderReconciler

		notifierNamespacedName := types.NamespacedName{
			Name:      "test-notifier",
			Namespace: "default",
		}

		configMapNotifierNamespacedName := types.NamespacedName{
			Name:      "webhook-config",
			Namespace: "default",
		}

		secretNotifierNamespacedName := types.NamespacedName{
			Name:      "webhook-secret",
			Namespace: "default",
		}

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
			By("creating the ConfigMap for the Notifier")
			err = k8sClient.Get(ctx, configMapNotifierNamespacedName, &corev1.ConfigMap{})
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapNotifierNamespacedName.Name,
						Namespace: configMapNotifierNamespacedName.Namespace,
					},
					Data: map[string]string{
						"config": "",
					},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the Secret for the Notifier")
			err = k8sClient.Get(ctx, secretNotifierNamespacedName, &corev1.Secret{})
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNotifierNamespacedName.Name,
						Namespace: secretNotifierNamespacedName.Namespace,
					},
					StringData: map[string]string{
						"url": "https://dummy.url",
					},
				}

				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the custom resource for the Kind Notifier")
			err = k8sClient.Get(ctx, notifierNamespacedName, notifier)
			if err != nil && errors.IsNotFound(err) {
				resource := &ddnsv1alpha1.Notifier{
					ObjectMeta: metav1.ObjectMeta{
						Name:      notifierNamespacedName.Name,
						Namespace: notifierNamespacedName.Namespace,
					},
					Spec: ddnsv1alpha1.NotifierSpec{
						Name:       "Webhook",
						SecretName: secretNotifierNamespacedName.Name,
						ConfigMap:  configMapNotifierNamespacedName.Name,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("creating the NotifierReconciler")
			controllerNotifierReconciler = &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				NotifierFactory: func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error) {
					return &MockNotifier{}, nil
				},
			}

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
						NotifierRefs: []ddnsv1alpha1.ResourceRef{
							{
								Name: notifierNamespacedName.Name,
							},
						},
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
				IPProvider: func(test string) (string, error) {
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

			notifierResource := &ddnsv1alpha1.Notifier{}
			secretNotifierResource := &corev1.Secret{}
			configMapNotifierResource := &corev1.ConfigMap{}

			Expect(k8sClient.Get(ctx, notifierNamespacedName, notifierResource)).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, secretNotifierNamespacedName, secretNotifierResource)).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, configMapNotifierNamespacedName, configMapNotifierResource)).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Notifier and related resources")
			Expect(k8sClient.Delete(ctx, notifierResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secretNotifierResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, configMapNotifierResource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			_, err := controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Marking the notifier as ready")
			resource := &ddnsv1alpha1.Notifier{}
			err = k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			Expect(resource.Status.Conditions).To(HaveLen(3))
			Expect(resource.Status.Conditions[0].Reason).To(Equal("ConfigMapFound"))
			Expect(resource.Status.Conditions[0].Type).To(Equal("ConfigMap"))
			Expect(resource.Status.Conditions[0].Message).To(Equal(fmt.Sprintf("ConfigMap %s found", configMapNotifierNamespacedName.Name)))
			Expect(resource.Status.Conditions[1].Reason).To(Equal("SecretFound"))
			Expect(resource.Status.Conditions[1].Type).To(Equal("Secret"))
			Expect(resource.Status.Conditions[1].Message).To(Equal(fmt.Sprintf("Secret %s found", secretNotifierNamespacedName.Name)))
			Expect(resource.Status.Conditions[2].Reason).To(Equal("ClientCommunication"))
			Expect(resource.Status.Conditions[2].Type).To(Equal("Client"))
			Expect(resource.Status.Conditions[2].Message).To(Equal("Communications established"))
			Expect(resource.Status.IsReady).To(BeTrue())
			Expect(int(resource.Status.ObservedGeneration)).To(Equal(0))
		})

		It("should not successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			controllerNotifierReconciler = &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				NotifierFactory: func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error) {
					return &MockNotifier{
						SendGreetingsError: fmt.Errorf("error sending greetings"),
					}, nil
				},
			}

			_, err := controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			By("Not marking the notifier as ready")
			resource := &ddnsv1alpha1.Notifier{}
			err = k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			Expect(resource.Status.Conditions).To(HaveLen(3))
			Expect(resource.Status.Conditions[0].Reason).To(Equal("ConfigMapFound"))
			Expect(resource.Status.Conditions[0].Type).To(Equal("ConfigMap"))
			Expect(resource.Status.Conditions[0].Message).To(Equal(fmt.Sprintf("ConfigMap %s found", configMapNotifierNamespacedName.Name)))
			Expect(resource.Status.Conditions[1].Reason).To(Equal("SecretFound"))
			Expect(resource.Status.Conditions[1].Type).To(Equal("Secret"))
			Expect(resource.Status.Conditions[1].Message).To(Equal(fmt.Sprintf("Secret %s found", secretNotifierNamespacedName.Name)))
			Expect(resource.Status.Conditions[2].Reason).To(Equal("ClientCommunication"))
			Expect(resource.Status.Conditions[2].Type).To(Equal("Client"))
			Expect(resource.Status.Conditions[2].Status).To(Equal(metav1.ConditionFalse))
			Expect(resource.Status.Conditions[2].Message).To(Equal("unable to send greetings: error sending greetings"))
			Expect(resource.Status.IsReady).NotTo(BeTrue())
			Expect(int(resource.Status.ObservedGeneration)).To(Equal(0))
		})

		It("should successfully reconcile the resource and not send a notification as the provider is not ready", func() {
			sendNotificationCounter := 0
			By("Creating a custom notifier reconciler")
			controllerNotifierReconciler = &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				NotifierFactory: func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error) {
					return &MockNotifier{
						SendNotificationInterceptor: func(message any) {
							sendNotificationCounter++
						},
					}, nil
				},
			}

			By("Reconciling the created resource")

			_, err = controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the resource has been successfully reconciled")
			resource := &ddnsv1alpha1.Notifier{}
			err = k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.IsReady).To(BeTrue())

			By("Ensuring a Provider resource exists")
			provider := &ddnsv1alpha1.Provider{}

			err = k8sClient.Get(ctx, providerNamespacedName, provider)
			if err != nil && errors.IsNotFound(err) {
				resource := &ddnsv1alpha1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      providerNamespacedName.Name,
						Namespace: providerNamespacedName.Namespace,
					},
					Spec: ddnsv1alpha1.ProviderSpec{
						Name:          "Cloudflare",
						SecretName:    secretNotifierNamespacedName.Name,
						ConfigMap:     configMapNamespacedName.Name,
						RetryInterval: 900,
						NotifierRefs:  []ddnsv1alpha1.ResourceRef{},
					},
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

			By("Sending a notification due to a change")
			_, err = controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sendNotificationCounter).To(Equal(0))
		})

		It("should successfully reconcile the resource and send a notification as the provider is ready", func() {
			sendNotificationCounter := 0
			By("Creating a custom notifier reconciler")
			controllerNotifierReconciler = &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				NotifierFactory: func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error) {
					return &MockNotifier{
						SendNotificationInterceptor: func(message any) {
							Expect(message).To(Equal(fmt.Sprintf("Provider IP (%s) in sync with Public IP. From provider: (%s).", dummyIp, providerNamespacedName.Name)))
							sendNotificationCounter++
						},
					}, nil
				},
			}

			By("Reconciling the created resource")

			_, err = controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the resource has been successfully reconciled")
			resource := &ddnsv1alpha1.Notifier{}
			err = k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.IsReady).To(BeTrue())

			By("Ensuring Provider is in correct state")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: providerNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Sending a notification due to a change")
			_, err = controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(sendNotificationCounter).To(Equal(1))
		})

		It("should successfully reconcile the resource and not send a notification as the provider is ready but there is an error", func() {
			sendNotificationCounter := 0
			By("Creating a custom notifier reconciler")
			controllerNotifierReconciler = &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				NotifierFactory: func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error) {
					return &MockNotifier{
						SendNotificationInterceptor: func(message any) {
							Expect(message).To(Equal(fmt.Sprintf("Provider IP (%s) in sync with Public IP. From provider: (%s).", dummyIp, providerNamespacedName.Name)))
							sendNotificationCounter++
						},
						SendNotificationError: fmt.Errorf("error sending notification"),
					}, nil
				},
			}

			By("Reconciling the created resource")

			_, err = controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the resource has been successfully reconciled")
			resource := &ddnsv1alpha1.Notifier{}
			err = k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.IsReady).To(BeTrue())

			By("Ensuring Provider is in correct state")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: providerNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Sending a notification due to a change")
			_, err = controllerNotifierReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			Expect(sendNotificationCounter).To(Equal(1))
		})
	})
})
