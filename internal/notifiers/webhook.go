package notifiers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type webhookData struct {
	Content string `json:"content"`
}

type WebhookNotifier struct {
	Url string
}

// SendGreetings sends a greeting message to the webhook
func (w *WebhookNotifier) SendGreetings() error {
	err := w.sendToWebhook("`go-ddns-controller` is starting its watch.")
	if err != nil {
		return err
	}

	return nil
}

// SendNotification sends a message to the webhook
func (w *WebhookNotifier) SendNotification(message any) error {
	if _, ok := message.(string); !ok {
		return fmt.Errorf("message is not a string")
	}

	err := w.sendToWebhook(message.(string))
	if err != nil {
		return err
	}

	return nil
}

func (w *WebhookNotifier) sendToWebhook(data string) error {
	webhookData := webhookData{
		Content: data,
	}

	var (
		requestBody []byte
		err         error
	)

	if requestBody, err = json.Marshal(webhookData); err != nil {
		return err
	}

	slog.Debug("Sending to webhook", "data", string(requestBody))

	resp, err := http.Post(w.Url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	slog.Debug("Status Code", "code", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var (
			err  error
			body []byte
		)
		if body, err = io.ReadAll(resp.Body); err == nil {
			return fmt.Errorf("error while trying to send to webhook. Error was %s", string(body))
		} else {
			return fmt.Errorf("error while parsing response from webhook. Error was %s", err)
		}
	}

	return nil
}
