package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	baseURL := "https://api.example.com"

	client := NewClient(baseURL)

	if client.BaseURL != baseURL {
		t.Errorf("Expected BaseURL to be %s, got %s", baseURL, client.BaseURL)
	}

	if client.token != "" {
		t.Errorf("Expected token to be empty, got %s", client.token)
	}

	if client.HTTPClient == nil {
		t.Error("Expected HTTPClient to be initialized")
	}

	// Check default timeout
	if client.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", client.HTTPClient.Timeout)
	}
}

func TestLogin(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "POST" {
			t.Errorf("Expected request method to be POST, got %s", r.Method)
		}

		// Check request path
		if r.URL.Path != "/console/api/login" {
			t.Errorf("Expected request path to be /console/api/login, got %s", r.URL.Path)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "success",
			"data": {
				"access_token": "test-access-token"
			}
		}`))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL)

	// Call the method
	err := client.Login("test@example.com", "password")

	// Check for errors
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check token was set
	if client.token != "test-access-token" {
		t.Errorf("Expected token to be 'test-access-token', got '%s'", client.token)
	}
}

func TestLoginErrors(t *testing.T) {
	// Test HTTP client error
	client := NewClient("invalid-url")
	err := client.Login("test@example.com", "password")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test non-200 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid credentials"}`))
	}))
	defer server.Close()

	client = NewClient(server.URL)
	err = client.Login("test@example.com", "wrong-password")
	if err == nil {
		t.Error("Expected error for 401 response")
	}

	// Test invalid JSON response
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client = NewClient(server.URL)
	err = client.Login("test@example.com", "password")
	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestGetAppInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "GET" {
			t.Errorf("Expected request method to be GET, got %s", r.Method)
		}

		// Check request path
		if r.URL.Path != "/console/api/apps/test-app-id" {
			t.Errorf("Expected request path to be /console/api/apps/test-app-id, got %s", r.URL.Path)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header to be 'Bearer test-token', got '%s'", auth)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": {
				"id": "test-app-id",
				"name": "Test App",
				"updated_at": "2023-01-01T12:00:00Z"
			}
		}`))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定

	// Call the method
	appInfo, err := client.GetAppInfo("test-app-id")

	// Check for errors
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check response
	if appInfo.ID != "test-app-id" {
		t.Errorf("Expected ID to be 'test-app-id', got '%s'", appInfo.ID)
	}

	if appInfo.Name != "Test App" {
		t.Errorf("Expected Name to be 'Test App', got '%s'", appInfo.Name)
	}

	expectedTime, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
	if !appInfo.UpdatedAt.Equal(expectedTime) {
		t.Errorf("Expected UpdatedAt to be %v, got %v", expectedTime, appInfo.UpdatedAt)
	}
}

func TestGetAppInfoErrors(t *testing.T) {
	// Test not authenticated error
	client := NewClient("https://api.example.com")
	_, err := client.GetAppInfo("test-app-id")
	if err == nil || err.Error() != "not authenticated, call Login() first" {
		t.Errorf("Expected 'not authenticated' error, got %v", err)
	}

	// Test HTTP client error
	client = NewClient("invalid-url")
	client.token = "test-token" // 直接トークンを設定
	_, err = client.GetAppInfo("test-app-id")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test non-200 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "App not found"}`))
	}))
	defer server.Close()

	client = NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定
	_, err = client.GetAppInfo("test-app-id")
	if err == nil {
		t.Error("Expected error for 404 response")
	}

	// Test invalid JSON response
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client = NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定
	_, err = client.GetAppInfo("test-app-id")
	if err == nil {
		t.Error("Expected error for invalid JSON response")
	}
}

func TestGetDSL(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "GET" {
			t.Errorf("Expected request method to be GET, got %s", r.Method)
		}

		// Check request path
		expectedPath := "/console/api/apps/test-app-id/export"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected request path to be %s, got %s", expectedPath, r.URL.Path)
		}

		// Check query parameter
		if r.URL.Query().Get("include_secret") != "true" {
			t.Errorf("Expected include_secret=true query parameter")
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header to be 'Bearer test-token', got '%s'", auth)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": "name: Test App\nversion: 1.0.0"
		}`))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定

	// Call the method
	dsl, err := client.GetDSL("test-app-id")

	// Check for errors
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check response
	expected := "name: Test App\nversion: 1.0.0"
	if string(dsl) != expected {
		t.Errorf("Expected DSL to be '%s', got '%s'", expected, string(dsl))
	}
}

func TestGetDSLErrors(t *testing.T) {
	// Test not authenticated error
	client := NewClient("https://api.example.com")
	_, err := client.GetDSL("test-app-id")
	if err == nil || err.Error() != "not authenticated, call Login() first" {
		t.Errorf("Expected 'not authenticated' error, got %v", err)
	}

	// Test HTTP client error
	client = NewClient("invalid-url")
	client.token = "test-token" // 直接トークンを設定
	_, err = client.GetDSL("test-app-id")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test non-200 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "DSL not found"}`))
	}))
	defer server.Close()

	client = NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定
	_, err = client.GetDSL("test-app-id")
	if err == nil {
		t.Error("Expected error for 404 response")
	}
}

func TestUpdateDSL(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "POST" {
			t.Errorf("Expected request method to be POST, got %s", r.Method)
		}

		// Check request path
		expectedPath := "/console/api/apps/test-app-id/import"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected request path to be %s, got %s", expectedPath, r.URL.Path)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header to be 'Bearer test-token', got '%s'", auth)
		}

		// Check content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/yaml" {
			t.Errorf("Expected Content-Type to be 'application/yaml', got '%s'", contentType)
		}

		// Check request body
		body, _ := io.ReadAll(r.Body)
		expected := "name: Test App\nversion: 1.0.0"
		if string(body) != expected {
			t.Errorf("Expected request body to be '%s', got '%s'", expected, string(body))
		}

		// Return a mock response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定

	// Call the method
	dsl := []byte("name: Test App\nversion: 1.0.0")
	err := client.UpdateDSL("test-app-id", dsl)

	// Check for errors
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestUpdateDSLErrors(t *testing.T) {
	// Test not authenticated error
	client := NewClient("https://api.example.com")
	err := client.UpdateDSL("test-app-id", []byte("test"))
	if err == nil || err.Error() != "not authenticated, call Login() first" {
		t.Errorf("Expected 'not authenticated' error, got %v", err)
	}

	// Test HTTP client error
	client = NewClient("invalid-url")
	client.token = "test-token" // 直接トークンを設定
	err = client.UpdateDSL("test-app-id", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Test request creation error (unlikely in practice but good for coverage)
	// This is a bit of a hack to trigger an error in http.NewRequest
	// by using an invalid request method
	client = &Client{
		BaseURL:    "http://example.com",
		token:      "test-token",
		HTTPClient: &http.Client{},
	}
	err = client.UpdateDSL("\000", []byte("test"))
	if err == nil {
		t.Error("Expected error for invalid app ID")
	}

	// Test non-200 response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Server error"}`))
	}))
	defer server.Close()

	client = NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定
	err = client.UpdateDSL("test-app-id", []byte("test"))
	if err == nil {
		t.Error("Expected error for 500 response")
	}
}

// TestGetAppList は GetAppList メソッドのテスト
func TestGetAppList(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "GET" {
			t.Errorf("Expected request method to be GET, got %s", r.Method)
		}

		// Check request path
		if r.URL.Path != "/console/api/apps" {
			t.Errorf("Expected request path to be /console/api/apps, got %s", r.URL.Path)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header to be 'Bearer test-token', got '%s'", auth)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": [
				{
					"id": "app-id-1",
					"name": "App 1",
					"updated_at": "2023-01-01T12:00:00Z"
				},
				{
					"id": "app-id-2",
					"name": "App 2",
					"updated_at": "2023-01-02T12:00:00Z"
				}
			]
		}`))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL)
	client.token = "test-token" // 直接トークンを設定

	// Call the method
	apps, err := client.GetAppList()

	// Check for errors
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check response
	if len(apps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(apps))
	}

	if apps[0].ID != "app-id-1" || apps[0].Name != "App 1" {
		t.Errorf("Expected first app to be App 1, got %+v", apps[0])
	}

	if apps[1].ID != "app-id-2" || apps[1].Name != "App 2" {
		t.Errorf("Expected second app to be App 2, got %+v", apps[1])
	}
}
