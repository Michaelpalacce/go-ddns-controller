package network

import (
	"fmt"
	"io"
	"net/http"
)

// GetBody does a Get request on the given url and returns the body in a []byte.
// Will also close the ReadStream
func GetBody(url string) ([]byte, error) {
	if resp, err := http.Get(url); err == nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fmt.Println("Error while closing response body")
			}
		}()

		if body, err := io.ReadAll(resp.Body); err == nil {
			return body, nil
		}
	}

	return nil, fmt.Errorf("http: Error while trying to fetch url: %s", url)
}
