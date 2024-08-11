package clients

var Cloudflare = "Cloudflare"

// Client is a general interface implemented by all clients
type Client interface {
	GetIp() (string, error)
	SetIp(ip string) error
}
