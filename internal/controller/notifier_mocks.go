package controller

import ddnsv1alpha1 "github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1"

type MockNotifier struct {
	SendGreetingsError          error
	SendNotificationError       error
	SendGreetingsInterceptor    func()
	SendNotificationInterceptor func()
}

func (n MockNotifier) SendGreetings(notifier *ddnsv1alpha1.Notifier) error {
	if n.SendGreetingsInterceptor != nil {
		n.SendGreetingsInterceptor()
	}
	return n.SendGreetingsError
}

func (n MockNotifier) SendNotification(message any) error {
	if n.SendNotificationInterceptor != nil {
		n.SendNotificationInterceptor()
	}
	return n.SendNotificationError
}
