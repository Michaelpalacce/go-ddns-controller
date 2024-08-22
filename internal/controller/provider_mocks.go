package controller

type MockClient struct {
	SetIPError       error
	GetIPError       error
	IP               string
	SetIPInterceptor func(string)
	GetIPInterceptor func()
}

func (c MockClient) GetIp() (string, error) {
	if c.GetIPInterceptor != nil {
		c.GetIPInterceptor()
	}
	return c.IP, c.GetIPError
}

func (c MockClient) SetIp(ip string) error {
	if c.SetIPInterceptor != nil {
		c.SetIPInterceptor(ip)
	}
	return c.SetIPError
}
