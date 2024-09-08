package network

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"golang.org/x/exp/rand"
)

// ipProviders is a list of providers that will be used to fetch the public IP
// Thanks to all the providers for providing this service.
// If you want a provider to be removed, please open an issue, I will respect your request.
var ipProviders = []string{
	"https://icanhazip.com", "https://api.ipify.org",
	"https://4.ident.me/", "https://api.seeip.org",
	"http://www.trackip.net/ip", "http://ifconfig.me",
}

// shuffle will shuffle the slice
func shuffle(slice []string) {
	rand.Seed(uint64(time.Now().UnixNano()))
	for i := len(slice) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// GetPublicIp will fetch the public IP of the
// machine that is running goip
func GetPublicIp(customIpProvider string) (string, error) {
	currentIpProviders := append(ipProviders, customIpProvider)

	shuffle(currentIpProviders)

	fmt.Println("ipProviders: ", currentIpProviders)

	for _, provider := range currentIpProviders {
		if provider == "" {
			continue
		}

		fmt.Println("provider: ", provider)
		ip, err := GetBody(provider)
		if err != nil {
			slog.Error("Error while trying to fetch ip from provider", "error", err, "provider", provider)
			continue
		}

		return net.ParseIP(strings.TrimSpace(string(ip))).String(), nil
	}

	return "", fmt.Errorf("could not retrieve a response from any of the providers")
}
