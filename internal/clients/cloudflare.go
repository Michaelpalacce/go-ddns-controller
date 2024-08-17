package clients

import (
	"context"
	"fmt"

	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
)

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

// CloudflareClient is the CloudflareClient client that will support Authentication and setting records
type CloudflareClient struct {
	API    *cloudflare.API
	Config CloudflareConfig
	Logger logr.Logger
}

// NewCloudflareClient creates a new CloudflareClient client
// It will return an error if the authentication fails
func NewCloudflareClient(config CloudflareConfig, apiToken string, logger logr.Logger) (*CloudflareClient, error) {
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

// GetIp returns the public IP from the first zone that has a record
func (c CloudflareClient) GetIp() (string, error) {
	for _, zone := range c.Config.Cloudflare.Zones {
		var (
			ip  string
			err error
		)

		if ip, err = c.getIpFromZone(zone); err != nil {
			c.Logger.Error(err, "Error while getting IP from zone, will search in next", "zone", zone.Name)
			continue
		}

		return ip, nil
	}

	return "", fmt.Errorf("error while trying to get IP from all zones")
}

// getIpFromZone returns the public IP for a specific zone
func (c CloudflareClient) getIpFromZone(zone Zone) (string, error) {
	zoneID, err := c.API.ZoneIDByName(zone.Name)
	if err != nil {
		return "", err
	}

	records, _, err := c.API.ListDNSRecords(context.Background(), cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{})
	if err != nil {
		return "", err
	}

	for _, r := range records {
		for _, zr := range zone.Records {
			if r.Type == "A" && r.Name == zr.Name {
				return r.Content, nil
			}
		}
	}
	return "", fmt.Errorf("could not find an A record for zone: %s", zone.Name)
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
