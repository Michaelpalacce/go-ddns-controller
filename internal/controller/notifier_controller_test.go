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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	notifierConditions "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/notifier/conditions"
	"github.com/Michaelpalacce/go-ddns-controller/internal/notifiers"
)

// Pending, need to be implemented
var _ = Describe("Notifier Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		notifier := &ddnsv1alpha1.Notifier{}
		ctx := context.Background()
		var controllerReconciler *NotifierReconciler

		notifierNamespacedName := types.NamespacedName{
			Name:      "test-notifier",
			Namespace: "default",
		}

		configMapNamespacedName := types.NamespacedName{
			Name:      "webhook-config",
			Namespace: "default",
		}

		secretNamespacedName := types.NamespacedName{
			Name:      "webhook-secret",
			Namespace: "default",
		}

		BeforeEach(func() {
			var err error

			By("creating the ConfigMap for the Notifier")
			err = k8sClient.Get(ctx, configMapNamespacedName, &corev1.ConfigMap{})
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      configMapNamespacedName.Name,
						Namespace: configMapNamespacedName.Namespace,
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
			err = k8sClient.Get(ctx, secretNamespacedName, &corev1.Secret{})
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNamespacedName.Name,
						Namespace: secretNamespacedName.Namespace,
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
						SecretName: secretNamespacedName.Name,
						ConfigMap:  configMapNamespacedName.Name,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			By("creating the ProviderReconciler")
			controllerReconciler = &NotifierReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				NotifierFactory: func(notifier *ddnsv1alpha1.Notifier, secret *corev1.Secret, configMap *corev1.ConfigMap) (notifiers.Notifier, error) {
					return &MockNotifier{}, nil
				},
			}
		})

		AfterEach(func() {
			resource := &ddnsv1alpha1.Notifier{}
			err := k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Notifier")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: notifierNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Checking if the resource has been successfully reconciled")
			resource := &ddnsv1alpha1.Notifier{}
			err = k8sClient.Get(ctx, notifierNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			Expect(resource.Status.Conditions).To(HaveLen(3))
			Expect(resource.Status.Conditions[0].Reason).To(Equal(notifierConditions.ConfigMapFound))
			Expect(resource.Status.Conditions[0].Type).To(Equal(notifierConditions.ConfigMapConditionType))
			Expect(resource.Status.Conditions[0].Message).To(Equal(fmt.Sprintf("ConfigMap %s found", configMapNamespacedName.Name)))
			Expect(resource.Status.Conditions[1].Reason).To(Equal(notifierConditions.SecretFound))
			Expect(resource.Status.Conditions[1].Type).To(Equal(notifierConditions.SecretConditionType))
			Expect(resource.Status.Conditions[1].Message).To(Equal(fmt.Sprintf("Secret %s found", secretNamespacedName.Name)))
			Expect(resource.Status.Conditions[2].Reason).To(Equal(notifierConditions.ClientCommunication))
			Expect(resource.Status.Conditions[2].Type).To(Equal(notifierConditions.ClientConditionType))
			Expect(resource.Status.Conditions[2].Message).To(Equal("Communications established"))
			Expect(resource.Status.IsReady).To(BeTrue())
			Expect(int(resource.Status.ObservedGeneration)).To(Equal(0))
			// Expect(resource.Status.ObservedGeneration)
		})
	})
})
