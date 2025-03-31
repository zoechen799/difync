package syncer

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppMap(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := ioutil.TempDir("", "difync-test-")
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
		Filename:  "test.yaml",
		AppID:     "test-app-id",
		Action:    ActionNone,
		Success:   true,
		Timestamp: time.Now(),
	}
	after := time.Now()

	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("Expected timestamp to be between %v and %v, got %v", before, after, result.Timestamp)
	}
}
