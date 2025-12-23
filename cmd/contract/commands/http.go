package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// HTTPClient wraps http.Client with common patterns for broker communication.
type HTTPClient struct {
	client  *http.Client
	baseURL string
}

// NewHTTPClient creates a new HTTP client with the broker base URL.
func NewHTTPClient(brokerURL string) *HTTPClient {
	return &HTTPClient{
		client:  &http.Client{},
		baseURL: brokerURL,
	}
}

// Get performs a GET request and returns the response body.
func (c *HTTPClient) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", string(body))
	}

	return body, nil
}

// GetJSON performs a GET and unmarshals the response into the target.
func (c *HTTPClient) GetJSON(ctx context.Context, path string, query url.Values, target any) error {
	body, err := c.Get(ctx, path, query)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

// PostJSON performs a POST with JSON body and returns the response body.
// expectedStatus specifies the expected HTTP status code (e.g., 200, 201).
func (c *HTTPClient) PostJSON(ctx context.Context, path string, body any, expectedStatus int) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != expectedStatus {
		return nil, fmt.Errorf("request failed: %s", string(respBody))
	}

	return respBody, nil
}

// PostJSONResult performs a POST and unmarshals the response into the target.
func (c *HTTPClient) PostJSONResult(ctx context.Context, path string, body any, expectedStatus int, target any) error {
	respBody, err := c.PostJSON(ctx, path, body, expectedStatus)
	if err != nil {
		return err
	}

	if target != nil {
		if err := json.Unmarshal(respBody, target); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}
