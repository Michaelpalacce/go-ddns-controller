package clients_test

import (
	. "github.com/onsi/ginkgo/v2"
	// . "github.com/onsi/gomega"
)

type MockLogger struct{}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {}

func (m *MockLogger) Error(err error, msg string, keysAndValues ...interface{}) {}

var _ = PDescribe("Cloudflare Client", func() {
	// var cloudflareClient *clients.CloudflareClient
	// BeforeEach(func() {
	// 	cloudflareClient = &clients.CloudflareClient{
	// 		Config: clients.CloudflareConfig{
	// 			Cloudflare: struct {
	// 				Zones []clients.Zone `json:"zones"`
	// 			}{
	// 				Zones: []clients.Zone{
	// 					{
	// 						Name: "example.com",
	// 						Records: []clients.Record{
	// 							{Name: "test", Proxied: false},
	// 						},
	// 					},
	// 				},
	// 			},
	// 		},
	// 		Logger: &MockLogger{},
	// 	}
	// })

	AfterEach(func() {
		// Your teardown code goes here
	})

	Describe("SetIp Should set the given IP in all the zones", func() {
	})
})
