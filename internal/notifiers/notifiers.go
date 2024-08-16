package notifiers

var Webhook = "Webhook"

// Notifier is an interface for sending notifications.
// All Notifiers should implement this interface
type Notifier interface {
	SendNotification(message any) error
	SendGreetings() error
}
