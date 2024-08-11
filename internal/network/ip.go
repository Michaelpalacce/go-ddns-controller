package network

import (
	"fmt"
	"log/slog"
	"strings"
)

// GetPublicIp will fetch the public IP of the
// machine that is running goip
func GetPublicIp() (string, error) {
	ipProviders := []string{"https://icanhazip.com"}

	for _, provider := range ipProviders {
		ip, err := GetBody(provider)
		if err != nil {
			slog.Error("Error while trying to fetch ip from provider", "error", err, "provider", provider)
			continue
		}

		return strings.Trim(string(ip), "\n"), nil
	}

	return "", fmt.Errorf("could not retrieve a response from any of the providers")
}
