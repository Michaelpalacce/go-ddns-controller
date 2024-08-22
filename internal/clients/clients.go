package clients

import (
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
)

var Cloudflare = "Cloudflare"

// Client is a general interface implemented by all clients
type Client interface {
	GetIp() (string, error)
	SetIp(ip string) error
}

// ClientFactory will return an authenticated, fully loaded client
func ClientFactory(name string, secret *corev1.Secret, configMap *corev1.ConfigMap, log logr.Logger) (Client, error) {
	var client Client
	switch name {
	case Cloudflare:
		var cloudflareConfig CloudflareConfig

		if configMap.Data["config"] == "" {
			return nil, fmt.Errorf("`config` not found in configMap")
		}

		configMap := configMap.Data["config"]

		err := json.Unmarshal([]byte(configMap), &cloudflareConfig)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal the config: %s", err)
		}

		if secret.Data["apiToken"] == nil {
			return nil, fmt.Errorf("`apiToken` not found in secret")
		}

		client, err = NewCloudflareClient(cloudflareConfig, string(secret.Data["apiToken"]), log)
		if err != nil {
			return nil, fmt.Errorf("could not create a Cloudflare client: %s", err)
		}
	default:
		return nil, fmt.Errorf("could not create a provider of type: %s", name)
	}

	return client, nil
}
