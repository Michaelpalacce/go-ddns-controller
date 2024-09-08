package clients_test

import (
	"context"

	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
	"github.com/cloudflare/cloudflare-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type MockLogger struct{}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {}

func (m *MockLogger) Error(err error, msg string, keysAndValues ...interface{}) {}

type MockAPI struct{}

func (m *MockAPI) ZoneIDByName(zoneName string) (string, error) {
	return "mock-zone-id", nil
}

func (m *MockAPI) ListDNSRecords(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
	return []cloudflare.DNSRecord{}, nil, nil
}

func (m *MockAPI) UpdateDNSRecord(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error) {
	return cloudflare.DNSRecord{}, nil
}

var _ = Describe("Cloudflare Client", func() {
	var cloudflareClient *clients.CloudflareClient
	BeforeEach(func() {
		cloudflareClient = &clients.CloudflareClient{
			Config: clients.CloudflareConfig{
				Cloudflare: struct {
					Zones []clients.Zone `json:"zones"`
				}{
					Zones: []clients.Zone{
						{
							Name: "example.com",
							Records: []clients.Record{
								{Name: "test", Proxied: false},
							},
						},
					},
				},
			},
			Logger: &MockLogger{},
			API:    &MockAPI{},
		}
	})

	AfterEach(func() {
		// Your teardown code goes here
	})

	Describe("SetIP", func() {
		It("Should set the IP in all the zones with no records", func() {
			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).To(BeNil())
			//
		})
	})
})
