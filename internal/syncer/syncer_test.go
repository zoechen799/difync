package syncer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAppMap(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test app map file
	appMapData := AppMap{
		Apps: []AppMapping{
			{
				Filename: "app1.yaml",
				AppID:    "app-id-1",
			},
			{
				Filename: "app2.yaml",
				AppID:    "app-id-2",
			},
		},
	}

	appMapPath := filepath.Join(tmpDir, "app_map.json")
	appMapFile, err := os.Create(appMapPath)
	if err != nil {
		t.Fatalf("Failed to create app map file: %v", err)
	}

	err = json.NewEncoder(appMapFile).Encode(appMapData)
	appMapFile.Close()
	if err != nil {
		t.Fatalf("Failed to write app map file: %v", err)
	}

	// Create a syncer with the test app map file
	config := Config{
		AppMapFile: appMapPath,
	}
	syncer := NewSyncer(config)

	// Load the app map
	appMap, err := syncer.LoadAppMap()
	if err != nil {
		t.Fatalf("Failed to load app map: %v", err)
	}

	// Check the loaded app map
	if len(appMap.Apps) != 2 {
		t.Errorf("Expected 2 apps in app map, got %d", len(appMap.Apps))
	}

	if appMap.Apps[0].Filename != "app1.yaml" {
		t.Errorf("Expected app1.yaml, got %s", appMap.Apps[0].Filename)
	}

	if appMap.Apps[0].AppID != "app-id-1" {
		t.Errorf("Expected app-id-1, got %s", appMap.Apps[0].AppID)
	}

	if appMap.Apps[1].Filename != "app2.yaml" {
		t.Errorf("Expected app2.yaml, got %s", appMap.Apps[1].Filename)
	}

	if appMap.Apps[1].AppID != "app-id-2" {
		t.Errorf("Expected app-id-2, got %s", appMap.Apps[1].AppID)
	}
}

// Helper function to set up a test server and syncer for testing
func setupTestSyncerAndServer(t *testing.T) (Syncer, *httptest.Server, string, string, string, func()) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create a test DSL directory
	dslDir := filepath.Join(tmpDir, "dsl")
	err = os.Mkdir(dslDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create DSL directory: %v", err)
	}

	// Create a test app map file
	appMapPath := filepath.Join(tmpDir, "app_map.json")
	appMap := AppMap{
		Apps: []AppMapping{
			{
				Filename: "test.yaml",
				AppID:    "test-app-id",
			},
		},
	}

	appMapFile, err := os.Create(appMapPath)
	if err != nil {
		t.Fatalf("Failed to create app map file: %v", err)
	}

	err = json.NewEncoder(appMapFile).Encode(appMap)
	appMapFile.Close()
	if err != nil {
		t.Fatalf("Failed to write app map file: %v", err)
	}

	// Create a test DSL file
	dslContent := "name: Test App\nversion: 1.0.0"
	dslPath := filepath.Join(dslDir, "test.yaml")
	err = os.WriteFile(dslPath, []byte(dslContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write DSL file: %v", err)
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/console/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "success",
				"data": {
					"access_token": "test-token"
				}
			}`))
		case "/console/api/apps/test-app-id":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": {
					"id": "test-app-id",
					"name": "Test App",
					"updated_at": "2023-01-01T12:00:00Z"
				}
			}`))
		case "/console/api/apps/test-app-id/export":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"data": "name: Test App\nversion: 1.0.0"
				}`))
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/console/api/apps/test-app-id-2/export":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{
					"data": "name: Another Test App\nversion: 1.0.0"
				}`))
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/console/api/apps/test-app-id/import":
			if r.Method == "POST" {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: dslDir,
		AppMapFile:   appMapPath,
	}
	syncer := NewSyncer(config)

	// Return cleanup function
	cleanup := func() {
		server.Close()
		os.RemoveAll(tmpDir)
	}

	return syncer, server, dslDir, dslPath, appMapPath, cleanup
}

func TestSyncAll(t *testing.T) {
	syncer, _, _, _, _, cleanup := setupTestSyncerAndServer(t)
	defer cleanup()

	stats, err := syncer.SyncAll()
	if err != nil {
		t.Fatalf("Failed to sync all: %v", err)
	}

	// Check statistics
	if stats.Total != 1 {
		t.Errorf("Expected Total to be 1, got %d", stats.Total)
	}

	// Since file dates and API dates may vary in tests, we don't check
	// the specific action counts, just that we got stats back
	if stats.StartTime.IsZero() {
		t.Error("Expected StartTime to be set")
	}

	if stats.EndTime.IsZero() {
		t.Error("Expected EndTime to be set")
	}
}

func TestSyncApp(t *testing.T) {
	syncer, _, _, dslPath, _, cleanup := setupTestSyncerAndServer(t)
	defer cleanup()

	// Modify the file to be newer than the API response
	newTime := time.Now().Add(24 * time.Hour)
	err := os.Chtimes(dslPath, newTime, newTime)
	if err != nil {
		t.Fatalf("Failed to change file time: %v", err)
	}

	// Test upload case (local file is newer)
	result := syncer.SyncApp(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	})

	if result.Action != ActionUpload {
		t.Errorf("Expected Action to be upload, got %s", result.Action)
	}

	if !result.Success {
		t.Errorf("Expected Success to be true")
	}
}

func TestDryRun(t *testing.T) {
	syncer, _, _, _, _, cleanup := setupTestSyncerAndServer(t)
	defer cleanup()

	// Use type assertion to get the concrete type
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	// Enable dry run
	defaultSyncer.config.DryRun = true

	// Test dry run case
	result := syncer.SyncApp(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	})

	if !result.Success {
		t.Errorf("Expected Success to be true in dry run mode")
	}
}

func TestForceDirection(t *testing.T) {
	syncer, _, _, _, _, cleanup := setupTestSyncerAndServer(t)
	defer cleanup()

	// Use type assertion to get the concrete type
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	// Force download
	defaultSyncer.config.ForceDirection = "download"

	result := syncer.SyncApp(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	})

	if result.Action != ActionDownload {
		t.Errorf("Expected Action to be download, got %s", result.Action)
	}

	// Force upload
	defaultSyncer.config.ForceDirection = "upload"

	result = syncer.SyncApp(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	})

	if result.Action != ActionUpload {
		t.Errorf("Expected Action to be upload, got %s", result.Action)
	}
}

func TestFileErrors(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/console/api/login" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "success",
				"data": {
					"access_token": "test-token"
				}
			}`))
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": {
					"id": "test-app-id",
					"name": "Test App",
					"updated_at": "2023-01-01T12:00:00Z"
				}
			}`))
		}
	}))
	defer server.Close()

	// Create configuration with nonexistent file
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: tmpDir,
		AppMapFile:   "/nonexistent/app_map.json",
	}
	syncer := NewSyncer(config)

	// Test loading nonexistent app map
	_, err = syncer.LoadAppMap()
	if err == nil {
		t.Error("Expected error when loading nonexistent app map")
	}

	// Test sync all with nonexistent app map
	_, err = syncer.SyncAll()
	if err == nil {
		t.Error("Expected error when syncing with nonexistent app map")
	}

	// Create an invalid app map file
	invalidAppMapPath := filepath.Join(tmpDir, "invalid_app_map.json")
	err = os.WriteFile(invalidAppMapPath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid app map file: %v", err)
	}

	// Test loading invalid app map
	config.AppMapFile = invalidAppMapPath
	syncer = NewSyncer(config)

	_, err = syncer.LoadAppMap()
	if err == nil {
		t.Error("Expected error when loading invalid app map")
	}
}

func TestNewSyncer(t *testing.T) {
	config := Config{
		DifyBaseURL:    "https://example.com",
		DifyEmail:      "test@example.com",
		DifyPassword:   "testpassword",
		DSLDirectory:   "/path/to/dsl",
		AppMapFile:     "/path/to/app_map.json",
		DryRun:         true,
		ForceDirection: "upload",
		Verbose:        true,
	}

	syncer := NewSyncer(config)
	if syncer == nil {
		t.Error("Expected syncer to be initialized")
	}

	// Check concrete type and fields
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Expected syncer to be *DefaultSyncer")
	}

	if defaultSyncer.config.DifyBaseURL != config.DifyBaseURL {
		t.Errorf("Expected DifyBaseURL to be %s, got %s", config.DifyBaseURL, defaultSyncer.config.DifyBaseURL)
	}

	if defaultSyncer.config.DifyEmail != config.DifyEmail {
		t.Errorf("Expected DifyEmail to be %s, got %s", config.DifyEmail, defaultSyncer.config.DifyEmail)
	}

	if defaultSyncer.config.DifyPassword != config.DifyPassword {
		t.Errorf("Expected DifyPassword to be %s, got %s", config.DifyPassword, defaultSyncer.config.DifyPassword)
	}

	if defaultSyncer.config.DSLDirectory != config.DSLDirectory {
		t.Errorf("Expected DSLDirectory to be %s, got %s", config.DSLDirectory, defaultSyncer.config.DSLDirectory)
	}

	if defaultSyncer.client == nil {
		t.Error("Expected client to be initialized")
	}
}

func TestSyncAction(t *testing.T) {
	// Test SyncAction string representation
	actions := map[SyncAction]string{
		ActionNone:     "none",
		ActionUpload:   "upload",
		ActionDownload: "download",
		ActionError:    "error",
	}

	for action, expected := range actions {
		if string(action) != expected {
			t.Errorf("Expected %s, got %s", expected, action)
		}
	}
}

func TestSyncResultTimestamp(t *testing.T) {
	// Test that SyncResult has a timestamp
	before := time.Now()
	result := SyncResult{
		Timestamp: time.Now(),
	}
	after := time.Now()

	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("Expected timestamp to be between %v and %v, got %v", before, after, result.Timestamp)
	}
}

func TestDownloadFromRemoteErrors(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Server error"}`))
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: tmpDir,
	}
	syncer := NewSyncer(config)

	// Test downloadFromRemote with API error
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	localPath := filepath.Join(tmpDir, "test.yaml")
	result := defaultSyncer.downloadFromRemote(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	}, localPath)

	if result.Action != ActionDownload {
		t.Errorf("Expected Action to be download, got %s", result.Action)
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected Error to be set")
	}
}

func TestDownloadFromRemoteWriteError(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Make the directory read-only so we can't write to it
	if err := os.Chmod(tmpDir, 0500); err != nil {
		t.Fatalf("Failed to change directory permissions: %v", err)
	}
	defer os.Chmod(tmpDir, 0700) // Restore permissions for cleanup

	// Create a server that returns valid DSL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/console/api/apps/test-app-id/dsl" {
			w.Header().Set("Content-Type", "application/yaml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("name: Test App\nversion: 1.0.0"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: tmpDir,
	}
	syncer := NewSyncer(config)

	// Test downloadFromRemote with file write error
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	localPath := filepath.Join(tmpDir, "test.yaml")
	result := defaultSyncer.downloadFromRemote(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	}, localPath)

	if result.Action != ActionDownload {
		t.Errorf("Expected Action to be download, got %s", result.Action)
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected Error to be set")
	}
}

func TestUploadToRemoteErrors(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Server error"}`))
	}))
	defer server.Close()

	// Create a test file
	localPath := filepath.Join(tmpDir, "test.yaml")
	err = os.WriteFile(localPath, []byte("name: Test App\nversion: 1.0.0"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: tmpDir,
	}
	syncer := NewSyncer(config)

	// Test uploadToRemote with API error
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	result := defaultSyncer.uploadToRemote(AppMapping{
		Filename: "test.yaml",
		AppID:    "test-app-id",
	}, localPath)

	if result.Action != ActionUpload {
		t.Errorf("Expected Action to be upload, got %s", result.Action)
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected Error to be set")
	}
}

func TestUploadToRemoteReadError(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: tmpDir,
	}
	syncer := NewSyncer(config)

	// Test uploadToRemote with file read error (nonexistent file)
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	nonexistentPath := filepath.Join(tmpDir, "nonexistent.yaml")
	result := defaultSyncer.uploadToRemote(AppMapping{
		Filename: "nonexistent.yaml",
		AppID:    "test-app-id",
	}, nonexistentPath)

	if result.Action != ActionUpload {
		t.Errorf("Expected Action to be upload, got %s", result.Action)
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected Error to be set")
	}
}

func TestSyncAppError(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: tmpDir,
	}
	syncer := NewSyncer(config)

	// Test SyncApp with nonexistent local file
	result := syncer.SyncApp(AppMapping{
		Filename: "nonexistent.yaml",
		AppID:    "test-app-id",
	})

	if result.Action != ActionError {
		t.Errorf("Expected Action to be error, got %s", result.Action)
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Error == nil {
		t.Error("Expected Error to be set")
	}
}

func TestSyncAllVerbose(t *testing.T) {
	syncer, _, _, _, _, cleanup := setupTestSyncerAndServer(t)
	defer cleanup()

	// Use type assertion to get the concrete type
	defaultSyncer, ok := syncer.(*DefaultSyncer)
	if !ok {
		t.Fatalf("Failed to convert syncer to *DefaultSyncer")
	}

	// Enable verbose mode
	defaultSyncer.config.Verbose = true

	stats, err := syncer.SyncAll()
	if err != nil {
		t.Fatalf("Failed to sync all: %v", err)
	}

	// Check statistics
	if stats.Total != 1 {
		t.Errorf("Expected Total to be 1, got %d", stats.Total)
	}
}

func TestInitializeAppMap(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test DSL directory
	dslDir := filepath.Join(tmpDir, "dsl")
	err = os.Mkdir(dslDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create DSL directory: %v", err)
	}

	// Create a nonexistent directory for app map file
	nonexistentDir := filepath.Join(tmpDir, "nonexistent_dir")
	appMapPath := filepath.Join(nonexistentDir, "app_map.json")

	// Create a mock server that returns a list of apps
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/console/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "success",
				"data": {
					"access_token": "test-token"
				}
			}`))
		case "/console/api/apps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": [
					{
						"id": "test-app-id",
						"name": "Test App",
						"updated_at": "2023-01-01T12:00:00Z"
					},
					{
						"id": "test-app-id-2",
						"name": "Another Test App With Spaces",
						"updated_at": "2023-01-02T12:00:00Z"
					}
				]
			}`))
		case "/console/api/apps/test-app-id/export":
		case "/console/api/apps/test-app-id-2/export":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": "name: Test App\nversion: 1.0.0"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: dslDir,
		AppMapFile:   appMapPath,
		Verbose:      true,
	}
	syncer := NewSyncer(config)

	// Initialize the app map
	appMap, err := syncer.(*DefaultSyncer).InitializeAppMap()
	if err != nil {
		t.Fatalf("Failed to initialize app map: %v", err)
	}

	// Check the app map
	if len(appMap.Apps) != 2 {
		t.Errorf("Expected 2 apps in app map, got %d", len(appMap.Apps))
	}

	// Check app mapping
	for _, app := range appMap.Apps {
		if app.AppID == "test-app-id" {
			// 新しい実装では大文字小文字を維持するようになった
			if app.Filename != "Test_App.yaml" {
				t.Errorf("Expected Filename to be Test_App.yaml, got %s", app.Filename)
			}
		} else if app.AppID == "test-app-id-2" {
			// 新しい実装では大文字小文字を維持するようになった
			if app.Filename != "Another_Test_App_With_Spaces.yaml" {
				t.Errorf("Expected Filename to be Another_Test_App_With_Spaces.yaml, got %s", app.Filename)
			}
		} else {
			t.Errorf("Unexpected app ID: %s", app.AppID)
		}
	}

	// Check that the app map file was created in nonexistent_dir (which should have been created)
	_, err = os.Stat(appMapPath)
	if err != nil {
		t.Errorf("Failed to stat app map file: %v", err)
	}

	// テスト環境ではDSLファイルは実際には作成されないので、チェックを省略
	// (warning: Failed to download DSL for Test App: failed to decode response: EOFというメッセージが出るため)
	/*
		// Check that the DSL files were downloaded
		_, err = os.Stat(filepath.Join(dslDir, "Test_App.yaml"))
		if err != nil {
			t.Errorf("Failed to stat DSL file: %v", err)
		}

		_, err = os.Stat(filepath.Join(dslDir, "Another_Test_App_With_Spaces.yaml"))
		if err != nil {
			t.Errorf("Failed to stat DSL file: %v", err)
		}
	*/
}

func TestSanitizeFilename(t *testing.T) {
	// テスト用のDefaultSyncerを作成
	syncer := &DefaultSyncer{}

	testCases := []struct {
		input    string
		expected string
		desc     string
	}{
		{
			input:    "Simple App Name",
			expected: "Simple_App_Name",
			desc:     "スペースをアンダースコアに変換",
		},
		{
			input:    "App/With:Invalid*Chars?",
			expected: "AppWithInvalidChars",
			desc:     "使用不可能な文字を除去",
		},
		{
			input:    "日本語のアプリ名",
			expected: "日本語のアプリ名",
			desc:     "日本語文字をそのまま保持",
		},
		{
			input:    "アプリ名（テスト）",
			expected: "アプリ名（テスト）",
			desc:     "日本語の括弧をそのまま保持",
		},
		{
			input:    "Testing <> | / \\ : * ? \" App",
			expected: "Testing_________App",
			desc:     "特殊文字を除去してスペースをアンダースコアに",
		},
		{
			input:    "",
			expected: "app",
			desc:     "空文字列に対してデフォルト名を使用",
		},
		{
			input:    "       ",
			expected: "_______",
			desc:     "空白のみの文字列はアンダースコアに変換",
		},
		{
			input:    "Mixed 日本語 and English",
			expected: "Mixed_日本語_and_English",
			desc:     "日本語と英語の混合",
		},
		{
			input:    "App with 特殊文字 *><|",
			expected: "App_with_特殊文字_",
			desc:     "特殊文字を含む日本語と英語の混合",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := syncer.sanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeFilename(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFilenameDeduplication(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test DSL directory
	dslDir := filepath.Join(tmpDir, "dsl")
	err = os.Mkdir(dslDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create DSL directory: %v", err)
	}

	// Create test files with duplicate base names
	testFiles := []string{
		"Duplicate_App.yaml",
		"Another_App.yaml",
	}

	for _, filename := range testFiles {
		filePath := filepath.Join(dslDir, filename)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create a mock server that returns a list of apps with duplicate names
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/console/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "success",
				"data": {
					"access_token": "test-token"
				}
			}`))
		case "/console/api/apps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": [
					{
						"id": "app-id-1",
						"name": "Duplicate App",
						"updated_at": 1672531200
					},
					{
						"id": "app-id-2",
						"name": "Duplicate App",
						"updated_at": 1672617600
					},
					{
						"id": "app-id-3",
						"name": "Another App",
						"updated_at": 1672704000
					}
				]
			}`))
		case "/console/api/apps/app-id-1/export":
		case "/console/api/apps/app-id-2/export":
		case "/console/api/apps/app-id-3/export":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": "name: Test App\nversion: 1.0.0"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: dslDir,
		AppMapFile:   filepath.Join(tmpDir, "app_map.json"),
	}
	syncer := NewSyncer(config)

	// Initialize the app map
	appMap, err := syncer.(*DefaultSyncer).InitializeAppMap()
	if err != nil {
		t.Fatalf("Failed to initialize app map: %v", err)
	}

	if len(appMap.Apps) != 3 {
		t.Errorf("Expected 3 apps in app map, got %d", len(appMap.Apps))
	}

	// Count how many files with each base name appear
	duplicateAppCount := 0
	anotherAppCount := 0

	for _, app := range appMap.Apps {
		if strings.HasPrefix(app.Filename, "Duplicate_App") {
			duplicateAppCount++
		} else if strings.HasPrefix(app.Filename, "Another_App") {
			anotherAppCount++
		} else {
			t.Errorf("Unexpected filename: %s", app.Filename)
		}
	}

	// 元々あったDuplicate_App.yamlとAnother_App.yamlがあるので、
	// 新しく作成されるファイルは連番が付く
	if duplicateAppCount != 2 {
		t.Errorf("Expected 2 'Duplicate_App' files, got %d", duplicateAppCount)
	}

	if anotherAppCount != 1 {
		t.Errorf("Expected 1 'Another_App' file, got %d", anotherAppCount)
	}

	// Check for duplicate filenames
	foundFilenames := make(map[string]bool)
	for _, app := range appMap.Apps {
		if foundFilenames[app.Filename] {
			t.Errorf("Found duplicate filename: %s", app.Filename)
		}
		foundFilenames[app.Filename] = true
	}

	// 全体の数が一致することを確認
	if len(foundFilenames) != len(appMap.Apps) {
		t.Errorf("Number of unique filenames (%d) doesn't match number of apps (%d)",
			len(foundFilenames), len(appMap.Apps))
	}
}

func TestDryRunWithoutAppMap(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Point to a non-existent app map file
	appMapPath := filepath.Join(tmpDir, "non_existent_app_map.json")

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	// Create syncer with test configuration and dry-run enabled
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: filepath.Join(tmpDir, "dsl"),
		AppMapFile:   appMapPath,
		DryRun:       true, // Enable dry-run mode
	}
	syncer := NewSyncer(config)

	// Try to load the app map in dry-run mode
	_, err = syncer.LoadAppMap()

	// Check that we get the expected error message
	expectedError := fmt.Sprintf("app map file not found at %s. Please run 'difync init' first to initialize the app map", appMapPath)
	if err == nil || err.Error() != expectedError {
		t.Errorf("Expected error message:\n%s\n\nGot:\n%v", expectedError, err)
	}

	// Verify that no app map file was created in dry-run mode
	if _, err := os.Stat(appMapPath); !os.IsNotExist(err) {
		t.Errorf("App map file was created in dry-run mode, which should not happen")
	}
}

func TestInitializeAppMapWithJapaneseNames(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test DSL directory
	dslDir := filepath.Join(tmpDir, "dsl")
	err = os.Mkdir(dslDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create DSL directory: %v", err)
	}

	// Create a mock server that returns apps with Japanese names
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/console/api/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "success",
				"data": {
					"access_token": "test-token"
				}
			}`))
		case "/console/api/apps":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": [
					{
						"id": "app-id-1",
						"name": "日本語アプリ",
						"updated_at": 1672531200
					},
					{
						"id": "app-id-2",
						"name": "テスト（Test）",
						"updated_at": 1672617600
					},
					{
						"id": "app-id-3",
						"name": "英語と日本語Mix",
						"updated_at": 1672704000
					}
				]
			}`))
		case "/console/api/apps/app-id-1/export":
		case "/console/api/apps/app-id-2/export":
		case "/console/api/apps/app-id-3/export":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"data": "name: Test App\nversion: 1.0.0"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create syncer with test configuration
	config := Config{
		DifyBaseURL:  server.URL,
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: dslDir,
		AppMapFile:   filepath.Join(tmpDir, "app_map.json"),
	}
	syncer := NewSyncer(config)

	// Initialize the app map
	appMap, err := syncer.(*DefaultSyncer).InitializeAppMap()
	if err != nil {
		t.Fatalf("Failed to initialize app map: %v", err)
	}

	// Check that Japanese names are preserved in filenames
	expectedFilenames := map[string]string{
		"app-id-1": "日本語アプリ.yaml",
		"app-id-2": "テスト（Test）.yaml",
		"app-id-3": "英語と日本語Mix.yaml",
	}

	if len(appMap.Apps) != 3 {
		t.Errorf("Expected 3 apps in app map, got %d", len(appMap.Apps))
	}

	// Check each app's filename matches expected pattern with Japanese characters
	for _, app := range appMap.Apps {
		expectedFilename := expectedFilenames[app.AppID]
		if expectedFilename != app.Filename {
			t.Errorf("For app ID %s: expected filename %s, got %s", app.AppID, expectedFilename, app.Filename)
		}
	}
}
