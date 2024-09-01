package notifiers

import (
	"fmt"

	ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var Webhook = "Webhook"

// Notifier is an interface for sending notifications.
// All Notifiers should implement this interface
type Notifier interface {
	SendNotification(message any) error
	SendGreetings(notifier *ddnsv1alpha1.Notifier) error
}

// NotifierFactory will return a Notifier based on the Notifier spec
func NotifierFactory(
	notifier *ddnsv1alpha1.Notifier,
	secret *corev1.Secret,
	configMap *corev1.ConfigMap,
) (Notifier, error) {
	switch notifier.Spec.Name {
	case Webhook:
		if secret.Data["url"] == nil {
			return nil, fmt.Errorf("`url` not found in secret")
		}

		return &WebhookNotifier{
			Url: string(secret.Data["url"]),
		}, nil
	default:
		return nil, fmt.Errorf("unknown notifier %s", notifier.Spec.Name)
	}
}
