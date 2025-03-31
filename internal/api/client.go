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
	HTTPClient *http.Client
	token      string // トークンをprivateフィールドに変更
}

// AppInfo represents the basic information about a Dify application
type AppInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginResponse represents the response from the login API
type LoginResponse struct {
	Status string `json:"status"`
	Data   struct {
		AccessToken string `json:"access_token"`
	} `json:"data"`
}

// NewClient creates a new Dify API client
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Login authenticates with Dify API using email and password
func (c *Client) Login(email, password string) error {
	url := fmt.Sprintf("%s/console/api/login", c.BaseURL)

	// Create login payload
	loginData := map[string]string{
		"email":    email,
		"password": password,
	}

	payload, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("failed to marshal login data: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login API returned error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	// Store the access token
	c.token = loginResp.Data.AccessToken
	return nil
}

// GetAppInfo fetches application information from Dify
func (c *Client) GetAppInfo(appID string) (*AppInfo, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps/%s", c.BaseURL, appID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
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
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps/%s/export?include_secret=true", c.BaseURL, appID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))

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
		Data string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return []byte(result.Data), nil
}

// UpdateDSL updates the DSL for a specific app in Dify
func (c *Client) UpdateDSL(appID string, dsl []byte) error {
	if c.token == "" {
		return fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps/%s/import", c.BaseURL, appID)

	req, err := http.NewRequest("POST", url, bytes.NewReader(dsl))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
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

// GetAppList fetches all applications from Dify
func (c *Client) GetAppList() ([]AppInfo, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps", c.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
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
		Data []AppInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}
