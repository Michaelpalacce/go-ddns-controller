package clients_test

import (
	"context"
	"fmt"

	"github.com/Michaelpalacce/go-ddns-controller/internal/clients"
	"github.com/cloudflare/cloudflare-go"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type MockLogger struct{}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {}

func (m *MockLogger) Error(err error, msg string, keysAndValues ...interface{}) {}

type MockAPI struct {
	ListDNSRecordsFunc  func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error)
	UpdateDNSRecordFunc func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error)
	ZoneIDByNameFunc    func(zoneName string) (string, error)
}

func (m *MockAPI) ZoneIDByName(zoneName string) (string, error) {
	if m.ZoneIDByNameFunc != nil {
		return m.ZoneIDByNameFunc(zoneName)
	}

	return "mock-zone-id", nil
}

func (m *MockAPI) ListDNSRecords(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
	if m.ListDNSRecordsFunc != nil {
		return m.ListDNSRecordsFunc(ctx, zoneID, params)
	}

	return []cloudflare.DNSRecord{}, nil, nil
}

func (m *MockAPI) UpdateDNSRecord(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error) {
	if m.UpdateDNSRecordFunc != nil {
		return m.UpdateDNSRecordFunc(ctx, zoneID, params)
	}

	return cloudflare.DNSRecord{}, nil
}

var _ = Describe("Cloudflare Client", func() {
	var cloudflareClient clients.CloudflareClient
	var cloudflareConfig clients.CloudflareConfig

	BeforeEach(func() {
		cloudflareConfig = clients.CloudflareConfig{
			Cloudflare: struct {
				Zones []clients.Zone `json:"zones"`
			}{
				Zones: []clients.Zone{
					{
						Name: "example.com",
						Records: []clients.Record{
							{Name: "test", Proxied: false},
							{Name: "test2", Proxied: false},
						},
					},
				},
			},
		}

		cloudflareClient = clients.CloudflareClient{
			Config: cloudflareConfig,
			Logger: &MockLogger{},
			API:    &MockAPI{},
		}
	})

	AfterEach(func() {
		// Your teardown code goes here
	})

	Describe("GetIP", func() {
		It("Should return the IP", func() {
			dummyIp := "127.0.0.1"
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: dummyIp,
							Type:    "A",
						},
						{
							Name:    "test2",
							Content: dummyIp + "1",
							Type:    "A",
						},
					}, nil, nil
				},
				ZoneIDByNameFunc: func(zoneName string) (string, error) {
					return "test", nil
				},
			}
			ip, err := cloudflareClient.GetIp()
			Expect(err).To(BeNil())
			Expect(ip).To(Equal(dummyIp))
		})

		It("Should return the IP after fallback", func() {
			dummyIp := "127.0.0.1"
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: dummyIp,
						},
						{
							Name:    "test2",
							Content: dummyIp + "1",
							Type:    "A",
						},
					}, nil, nil
				},
				ZoneIDByNameFunc: func(zoneName string) (string, error) {
					return "test", nil
				},
			}
			ip, err := cloudflareClient.GetIp()
			Expect(err).To(BeNil())
			Expect(ip).To(Equal(dummyIp + "1"))
		})

		It("Should return an err if cannot find ip", func() {
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: "",
						},
					}, nil, nil
				},
			}
			_, err := cloudflareClient.GetIp()
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("error while trying to get IP from all zones"))
		})
	})

	Describe("SetIP", func() {
		It("Should set the IP in all the zones with no records", func() {
			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).To(BeNil())
		})

		It("Should set the IP in all the zones with records", func() {
			callCount := 0
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: "",
						},
					}, nil, nil
				},
				UpdateDNSRecordFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error) {
					callCount++

					return cloudflare.DNSRecord{}, nil
				},
			}

			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).To(BeNil())
			Expect(callCount).To(Equal(1))
		})

		It("Should set the IP in all the zones", func() {
			callCount := 0
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: "",
						},
						{
							Name:    "test2",
							Content: "",
						},
					}, nil, nil
				},
				UpdateDNSRecordFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error) {
					callCount++

					return cloudflare.DNSRecord{}, nil
				},
			}

			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).To(BeNil())
			Expect(callCount).To(Equal(2))
		})

		It("Should set the IP in all zones that are present in the configuration only", func() {
			callCount := 0
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: "",
						},
						{
							Name:    "does-not-exist",
							Content: "",
						},
					}, nil, nil
				},
				UpdateDNSRecordFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error) {
					callCount++

					return cloudflare.DNSRecord{}, nil
				},
			}

			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).To(BeNil())
			Expect(callCount).To(Equal(1))
		})

		It("Should return err if ZoneByIP returns an err", func() {
			cloudflareClient.API = &MockAPI{
				ZoneIDByNameFunc: func(zoneName string) (string, error) {
					return "", fmt.Errorf("zone not found")
				},
			}
			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("zone not found"))
		})

		It("Should return err if listing dns records returns an err", func() {
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return nil, nil, fmt.Errorf("error listing dns records")
				},
			}
			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("error listing dns records"))
		})

		It("Should return err if UpdateDNSRecord returns an err", func() {
			cloudflareClient.API = &MockAPI{
				ListDNSRecordsFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.ListDNSRecordsParams) ([]cloudflare.DNSRecord, *cloudflare.ResultInfo, error) {
					return []cloudflare.DNSRecord{
						{
							Name:    "test",
							Content: "",
						},
					}, nil, nil
				},
				UpdateDNSRecordFunc: func(ctx context.Context, zoneID *cloudflare.ResourceContainer, params cloudflare.UpdateDNSRecordParams) (cloudflare.DNSRecord, error) {
					return cloudflare.DNSRecord{}, fmt.Errorf("error updating dns record")
				},
			}
			err := cloudflareClient.SetIp("127.0.0.1")
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(Equal("error updating dns record"))
		})
	})
})
