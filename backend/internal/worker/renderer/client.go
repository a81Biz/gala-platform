package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client interface {
	Render(spec any) error
	RenderV1(spec any) error
}

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (c *HTTPClient) Render(spec any) error {
	return c.post("/render", spec)
}

func (c *HTTPClient) RenderV1(spec any) error {
	return c.post("/render/v1", spec)
}

func (c *HTTPClient) post(path string, spec any) error {
	body, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("renderer http %d", res.StatusCode)
	}
	return nil
}
