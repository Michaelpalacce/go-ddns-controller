package network

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultClient is used as we want to set a timeout for the http requests
var defaultClient = http.Client{
	Timeout: time.Second * 1,
}

// GetBody does a Get request on the given url and returns the body in a []byte.
// Will also close the ReadStream
func GetBody(url string) ([]byte, error) {
	if resp, err := defaultClient.Get(url); err == nil {
		defer resp.Body.Close()

		if body, err := io.ReadAll(resp.Body); err == nil {
			return body, nil
		}
	}

	return nil, fmt.Errorf("http: Error while trying to fetch url: %s", url)
}
