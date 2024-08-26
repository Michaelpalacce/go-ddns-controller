package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

type ClientWrapper struct {
	client.Client

	PatchStatusError   error
	PatchStatusIndex   int // When to fail the PatchStatus
	CurrentStatusIndex int

	GetError        error
	GetIndex        int
	CurrentGetIndex int
}

func (c *ClientWrapper) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.GetError != nil {
		if c.CurrentGetIndex == c.GetIndex {
			return c.GetError
		}

		c.CurrentGetIndex++
	}

	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *ClientWrapper) Status() client.StatusWriter {
	wrapper := &StatusWriterWrapper{
		StatusWriter:       c.Client.Status(),
		PatchStatusError:   c.PatchStatusError,
		PatchStatusIndex:   c.PatchStatusIndex,
		CurrentStatusIndex: c.CurrentStatusIndex,
	}
	c.CurrentStatusIndex++
	return wrapper
}

type StatusWriterWrapper struct {
	client.StatusWriter
	PatchStatusError   error
	PatchStatusIndex   int
	CurrentStatusIndex int
}

func (s *StatusWriterWrapper) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if s.PatchStatusError != nil {
		if s.CurrentStatusIndex == s.PatchStatusIndex {
			return s.PatchStatusError
		}
	}

	return s.StatusWriter.Patch(ctx, obj, patch, opts...)
}
