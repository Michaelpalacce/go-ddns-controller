package clients

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go"
)

type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}

// Record represents one Zone Record
type Record struct {
	Name    string `json:"name"`
	Proxied bool   `json:"proxied"`
}

// Zone (s) are how Cloudflare separates different DNS endpoints
type Zone struct {
	Name    string   `json:"name"`
	Records []Record `json:"records"`
}

// CloudflareConfig is the structure of the json config that is expected
type CloudflareConfig struct {
	Cloudflare struct {
		Zones []Zone `json:"zones"`
	} `json:"cloudflare"`
}

type CloudflareSecret struct {
	APIToken string `json:"apiToken"`
}

type cloudflareApi interface {
	ZoneIDByName(zoneName string) (string, error)
	ListDNSRecords(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error)
	UpdateDNSRecord(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error)
}

// CloudflareClient is the CloudflareClient client that will support Authentication and setting records
type CloudflareClient struct {
	API    cloudflareApi
	Config CloudflareConfig
	Logger Logger
}

// NewCloudflareClient creates a new CloudflareClient client
// It will return an error if the authentication fails
func NewCloudflareClient(config CloudflareConfig, apiToken string, logger Logger) (*CloudflareClient, error) {
	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		return nil, fmt.Errorf("could not authenticate to Cloudflare with the given token, error was: %s", err)
	}

	return &CloudflareClient{
		Config: config,
		API:    api,
		Logger: logger,
	}, nil
}

// SetIp sets the IP for the given zones based on the configuration
func (c CloudflareClient) SetIp(ip string) error {
	for _, zone := range c.Config.Cloudflare.Zones {
		c.Logger.Info("Setting IP for zone", "zone", zone.Name)

		if err := c.setIpForZone(ip, zone); err != nil {
			return err
		}
	}

	return nil
}

// GetIp returns the public IP from all the zones
func (c CloudflareClient) GetIp() ([]string, error) {
	ips := make([]string, 0)

	for _, zone := range c.Config.Cloudflare.Zones {
		var err error

		if ips, err = c.getIpsFromZone(zone); err != nil {
			return nil, err
		}
	}

	return ips, nil
}

// getIpFromZone returns the public IPs for a records in a specific zone
func (c CloudflareClient) getIpsFromZone(zone Zone) ([]string, error) {
	ips := make([]string, 0)
	zoneID, err := c.API.ZoneIDByName(zone.Name)
	if err != nil {
		return ips, err
	}

	records, _, err := c.API.ListDNSRecords(context.Background(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return ips, err
	}

	for _, r := range records {
		for _, zr := range zone.Records {
			if r.Type == "A" && r.Name == zr.Name {
				ips = append(ips, r.Content)
			}
		}
	}
	return ips, nil
}

// setIpForZone sets the public ip for a specific zone
func (c CloudflareClient) setIpForZone(ip string, zone Zone) error {
	zoneID, err := c.API.ZoneIDByName(zone.Name)
	if err != nil {
		return err
	}
	c.Logger.Info("Found zone", "zoneId", zoneID, "zoneName", zone.Name)

	for _, r := range zone.Records {
		c.Logger.Info("Setting IP for record", "record", r)
		if err := c.setIpForRecord(ip, zoneID, r); err != nil {
			return err
		}
	}

	return nil
}

// setIpForRecord will update the specific record
func (c CloudflareClient) setIpForRecord(ip string, zoneID string, record Record) error {
	records, _, err := c.API.ListDNSRecords(context.Background(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return err
	}

	for _, r := range records {
		if r.Name == record.Name {
			c.Logger.Info("Updating record", "recordName", record.Name)

			_, err := c.API.UpdateDNSRecord(context.Background(), cloudflare.ZoneIdentifier(zoneID), cloudflare.UpdateDNSRecordParams{
				ID:      r.ID,
				Content: ip,
				Proxied: cloudflare.BoolPtr(record.Proxied),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
