package notifiers

import "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"

var Webhook = "Webhook"

// Notifier is an interface for sending notifications.
// All Notifiers should implement this interface
type Notifier interface {
	SendNotification(message any) error
	SendGreetings(notifier *v1alpha1.Notifier) error
}
