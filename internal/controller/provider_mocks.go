package controller

type MockClient struct {
	SetIpError error
	GetIpError error
	IP         string
}

func (c MockClient) GetIp() (string, error) {
	return c.IP, c.GetIpError
}

func (c MockClient) SetIp(ip string) error {
	return c.SetIpError
}
