// Package api provides a client for interacting with Dify.AI API
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a Dify API client
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// AppInfo represents the basic information about a Dify application
type AppInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewClient creates a new Dify API client
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetAppInfo fetches application information from Dify
func (c *Client) GetAppInfo(appID string) (*AppInfo, error) {
	url := fmt.Sprintf("%s/console/api/apps/%s", c.BaseURL, appID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		Data AppInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}

// GetDSL fetches the DSL for a specific app from Dify
func (c *Client) GetDSL(appID string) ([]byte, error) {
	url := fmt.Sprintf("%s/console/api/apps/%s/dsl", c.BaseURL, appID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// UpdateDSL updates the DSL for a specific app in Dify
func (c *Client) UpdateDSL(appID string, dsl []byte) error {
	url := fmt.Sprintf("%s/console/api/apps/%s/dsl", c.BaseURL, appID)

	req, err := http.NewRequest("POST", url, bytes.NewReader(dsl))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	req.Header.Set("Content-Type", "application/yaml")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}
