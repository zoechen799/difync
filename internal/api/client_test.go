package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	baseURL := "https://api.example.com"
	token := "test-token"

	client := NewClient(baseURL, token)

	if client.BaseURL != baseURL {
		t.Errorf("Expected BaseURL to be %s, got %s", baseURL, client.BaseURL)
	}

	if client.Token != token {
		t.Errorf("Expected Token to be %s, got %s", token, client.Token)
	}

	if client.HTTPClient == nil {
		t.Error("Expected HTTPClient to be initialized")
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
	client := NewClient(server.URL, "test-token")

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

func TestGetDSL(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "GET" {
			t.Errorf("Expected request method to be GET, got %s", r.Method)
		}

		// Check request path
		if r.URL.Path != "/console/api/apps/test-app-id/dsl" {
			t.Errorf("Expected request path to be /console/api/apps/test-app-id/dsl, got %s", r.URL.Path)
		}

		// Check authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header to be 'Bearer test-token', got '%s'", auth)
		}

		// Return a mock response
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("name: Test App\nversion: 1.0.0"))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL, "test-token")

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

func TestUpdateDSL(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method
		if r.Method != "POST" {
			t.Errorf("Expected request method to be POST, got %s", r.Method)
		}

		// Check request path
		if r.URL.Path != "/console/api/apps/test-app-id/dsl" {
			t.Errorf("Expected request path to be /console/api/apps/test-app-id/dsl, got %s", r.URL.Path)
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

		// Return a mock response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(server.URL, "test-token")

	// Call the method
	dsl := []byte("name: Test App\nversion: 1.0.0")
	err := client.UpdateDSL("test-app-id", dsl)

	// Check for errors
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}
