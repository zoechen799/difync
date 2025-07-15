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
	token      string // Changed token to private field
}

// AppInfo represents the basic information about a Dify application
type AppInfo struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	UpdatedAt interface{} `json:"updated_at"` // Changed to interface{} to handle both string and numeric types
}

// AppPublishInfo represents the publish information about a Dify application
type AppPublishInfo struct {
	ID        string      `json:"id"`
	Version   string      `json:"version"`
	UpdatedAt interface{} `json:"updated_at"`
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

	// Save response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug output
	fmt.Printf("Debug - Raw API Response: %s\n", string(body))

	// Decode JSON directly to map to avoid mapping issues
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("failed to decode JSON to map: %w", err)
	}

	// Check for data field first
	dataField, hasData := rawData["data"]
	if hasData {
		// If there's a data field, use that as our appData
		if appData, ok := dataField.(map[string]interface{}); ok {
			appInfo := &AppInfo{}
			// Set ID and Name
			if id, ok := appData["id"].(string); ok {
				appInfo.ID = id
			}
			if name, ok := appData["name"].(string); ok {
				appInfo.Name = name
			}
			// Get and set updated_at directly
			if updatedAt, exists := appData["updated_at"]; exists {
				appInfo.UpdatedAt = updatedAt
				fmt.Printf("Debug - Found updated_at in data: %v (type: %T)\n", updatedAt, updatedAt)
			} else {
				fmt.Printf("Debug - updated_at field not found in data\n")
			}
			fmt.Printf("Debug - Constructed AppInfo from data: %+v\n", appInfo)
			return appInfo, nil
		}
	}

	// Fallback to checking top-level fields (for backward compatibility)
	appInfo := &AppInfo{}

	// Set ID and Name from top-level
	if id, ok := rawData["id"].(string); ok {
		appInfo.ID = id
	}
	if name, ok := rawData["name"].(string); ok {
		appInfo.Name = name
	}

	// Get and set updated_at directly from top-level
	if updatedAt, exists := rawData["updated_at"]; exists {
		appInfo.UpdatedAt = updatedAt
		fmt.Printf("Debug - Found updated_at in raw response: %v (type: %T)\n", updatedAt, updatedAt)
	} else {
		fmt.Printf("Debug - updated_at field not found in response\n")
	}

	fmt.Printf("Debug - Constructed AppInfo: %+v\n", appInfo)
	return appInfo, nil
}

// GetAppPublish fetches application publish information from Dify
func (c *Client) GetAppPublish(appID string) (*AppPublishInfo, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps/%s/workflows/publish", c.BaseURL, appID)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Debug - GetAppPublish Raw API Response: %s\n", string(body))

	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("failed to decode JSON to map: %w", err)
	}

	appPublishInfo := &AppPublishInfo{}
	if id, ok := rawData["id"].(string); ok {
		appPublishInfo.ID = id
	}
	if version, ok := rawData["version"].(string); ok {
		appPublishInfo.Version = version
	}
	if updatedAt, exists := rawData["updated_at"]; exists {
		appPublishInfo.UpdatedAt = updatedAt
	}

	fmt.Printf("Debug - Constructed AppPublishInfo: %+v\n", appPublishInfo)
	return appPublishInfo, nil
}

// GetDSL fetches the DSL for a specific app from Dify
func (c *Client) GetDSL(appID string) ([]byte, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps/%s/export?include_secret=false", c.BaseURL, appID)

	fmt.Printf("Debug - Using export URL: %s\n", url)

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
		return nil, fmt.Errorf("API returned error: status=%d, url=%s, body=%s", resp.StatusCode, url, string(body))
	}

	var result struct {
		Data string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return []byte(result.Data), nil
}

// DoesDSLExist checks if a DSL exists in Dify for the given app ID
func (c *Client) DoesDSLExist(appID string) (bool, error) {
	if c.token == "" {
		return false, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps/%s", c.BaseURL, appID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// If status is 404, app doesn't exist
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	// If status is not 200 or 404, there was an error
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("API returned error: status=%d, url=%s, body=%s", resp.StatusCode, url, string(body))
	}

	// App exists
	return true, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetAppList fetches all applications from Dify
func (c *Client) GetAppList() ([]AppInfo, error) {
	if c.token == "" {
		return nil, fmt.Errorf("not authenticated, call Login() first")
	}

	url := fmt.Sprintf("%s/console/api/apps", c.BaseURL)

	fmt.Printf("Debug - Using app list URL: %s\n", url)

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
		return nil, fmt.Errorf("API returned error: status=%d, url=%s, body=%s", resp.StatusCode, url, string(body))
	}

	// Save response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug output
	fmt.Printf("Debug - GetAppList Raw API Response: %s\n", string(body))

	// New implementation: use map for more flexible parsing
	var rawData map[string]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("failed to decode JSON to map: %w", err)
	}

	// Get data array
	dataInterface, hasData := rawData["data"]
	if !hasData {
		return nil, fmt.Errorf("API response does not contain 'data' field")
	}

	dataArray, isArray := dataInterface.([]interface{})
	if !isArray {
		return nil, fmt.Errorf("API response 'data' is not an array")
	}

	// Create app info slice
	apps := make([]AppInfo, 0, len(dataArray))

	// Get each app's information
	for _, item := range dataArray {
		appData, isMap := item.(map[string]interface{})
		if !isMap {
			continue
		}

		app := AppInfo{}

		// Set each field
		if id, ok := appData["id"].(string); ok {
			app.ID = id
		}

		if name, ok := appData["name"].(string); ok {
			app.Name = name
		}

		// Get updated_at directly
		if updatedAt, exists := appData["updated_at"]; exists {
			app.UpdatedAt = updatedAt
		}

		apps = append(apps, app)
	}

	fmt.Printf("Debug - Parsed %d apps from response\n", len(apps))
	return apps, nil
}
