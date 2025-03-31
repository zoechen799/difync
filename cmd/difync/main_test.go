package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pepabo/difync/internal/syncer"
)

func TestGetEnvWithDefault(t *testing.T) {
	// Test when environment variable is set
	key := "TEST_ENV_VAR"
	expectedValue := "test-value"
	os.Setenv(key, expectedValue)
	defer os.Unsetenv(key)

	value := getEnvWithDefault(key, "default-value")
	if value != expectedValue {
		t.Errorf("Expected '%s', got '%s'", expectedValue, value)
	}

	// Test when environment variable is not set
	unsetKey := "UNSET_TEST_ENV_VAR"
	os.Unsetenv(unsetKey) // Make sure it's not set
	defaultValue := "default-value"
	value = getEnvWithDefault(unsetKey, defaultValue)
	if value != defaultValue {
		t.Errorf("Expected '%s', got '%s'", defaultValue, value)
	}
}

// Test flags
// This is a bit tricky since flags are package level variables
// We need to reset them after the test
func TestFlags(t *testing.T) {
	// Save old flag values to restore after test
	oldFlagSet := flag.CommandLine
	defer func() {
		flag.CommandLine = oldFlagSet
	}()

	// Reset flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Register flags again
	// These must match the flags defined in main.go
	baseURL := flag.String("base-url", "", "Dify API base URL (overrides env: DIFY_BASE_URL)")
	dslDir := flag.String("dsl-dir", "", "Directory containing DSL files (overrides env: DSL_DIRECTORY, default: dsl)")
	appMapFile := flag.String("app-map", "", "Path to app mapping file (overrides env: APP_MAP_FILE, default: app_map.json)")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run without making any changes")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	// Parse test args
	err := flag.CommandLine.Parse([]string{
		"-base-url", "https://test.example.com",
		"-dsl-dir", "test-dsl",
		"-app-map", "test-map.json",
		"-dry-run",
		"-verbose",
	})
	if err != nil {
		t.Fatalf("Failed to parse flags: %v", err)
	}

	// Verify flag values
	if *baseURL != "https://test.example.com" {
		t.Errorf("Expected base-url to be 'https://test.example.com', got '%s'", *baseURL)
	}

	if *dslDir != "test-dsl" {
		t.Errorf("Expected dsl-dir to be 'test-dsl', got '%s'", *dslDir)
	}

	if *appMapFile != "test-map.json" {
		t.Errorf("Expected app-map to be 'test-map.json', got '%s'", *appMapFile)
	}

	if !*dryRun {
		t.Errorf("Expected dry-run to be true")
	}

	if !*verbose {
		t.Errorf("Expected verbose to be true")
	}
}

func TestLoadConfigAndValidate(t *testing.T) {
	// Save old flags and environment variables to restore later
	oldFlagSet := flag.CommandLine
	oldBaseURL := os.Getenv("DIFY_BASE_URL")
	oldEmail := os.Getenv("DIFY_EMAIL")
	oldPassword := os.Getenv("DIFY_PASSWORD")
	oldDSLDir := os.Getenv("DSL_DIRECTORY")
	oldAppMapFile := os.Getenv("APP_MAP_FILE")

	defer func() {
		flag.CommandLine = oldFlagSet
		os.Setenv("DIFY_BASE_URL", oldBaseURL)
		os.Setenv("DIFY_EMAIL", oldEmail)
		os.Setenv("DIFY_PASSWORD", oldPassword)
		os.Setenv("DSL_DIRECTORY", oldDSLDir)
		os.Setenv("APP_MAP_FILE", oldAppMapFile)
	}()

	// Reset flags and environment variables
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Unsetenv("DIFY_BASE_URL")
	os.Unsetenv("DIFY_EMAIL")
	os.Unsetenv("DIFY_PASSWORD")
	os.Unsetenv("DSL_DIRECTORY")
	os.Unsetenv("APP_MAP_FILE")

	// Register flags again
	difyBaseURL = flag.String("base-url", "", "Dify API base URL (overrides env: DIFY_BASE_URL)")
	dslDir = flag.String("dsl-dir", "", "Directory containing DSL files (overrides env: DSL_DIRECTORY, default: dsl)")
	appMapFile = flag.String("app-map", "", "Path to app mapping file (overrides env: APP_MAP_FILE, default: app_map.json)")
	dryRun = flag.Bool("dry-run", false, "Perform a dry run without making any changes")
	verbose = flag.Bool("verbose", false, "Enable verbose output")

	// Test with missing required parameters
	flag.CommandLine.Parse([]string{})
	_, err := loadConfigAndValidate()
	if err == nil {
		t.Error("Expected error for missing base URL, email and password")
	}

	// Test with base URL but missing email/password
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	difyBaseURL = flag.String("base-url", "", "")
	dslDir = flag.String("dsl-dir", "", "")
	appMapFile = flag.String("app-map", "", "")
	dryRun = flag.Bool("dry-run", false, "")
	verbose = flag.Bool("verbose", false, "")

	flag.CommandLine.Parse([]string{"-base-url", "https://test.example.com"})
	_, err = loadConfigAndValidate()
	if err == nil {
		t.Error("Expected error for missing email/password")
	}

	// Test with valid parameters from flags and env
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	difyBaseURL = flag.String("base-url", "", "")
	dslDir = flag.String("dsl-dir", "", "")
	appMapFile = flag.String("app-map", "", "")
	dryRun = flag.Bool("dry-run", false, "")
	verbose = flag.Bool("verbose", false, "")

	flag.CommandLine.Parse([]string{
		"-base-url", "https://test.example.com",
		"-dry-run",
	})

	// Set environment variables
	os.Setenv("DIFY_EMAIL", "test@example.com")
	os.Setenv("DIFY_PASSWORD", "testpassword")

	config, err := loadConfigAndValidate()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config.DifyBaseURL != "https://test.example.com" {
		t.Errorf("Expected DifyBaseURL to be 'https://test.example.com', got '%s'", config.DifyBaseURL)
	}

	if config.DifyEmail != "test@example.com" {
		t.Errorf("Expected DifyEmail to be 'test@example.com', got '%s'", config.DifyEmail)
	}

	if config.DifyPassword != "testpassword" {
		t.Errorf("Expected DifyPassword to be 'testpassword', got '%s'", config.DifyPassword)
	}

	if !config.DryRun {
		t.Errorf("Expected DryRun to be true")
	}

	// Test with valid parameters from environment variables
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	difyBaseURL = flag.String("base-url", "", "")
	dslDir = flag.String("dsl-dir", "", "")
	appMapFile = flag.String("app-map", "", "")
	dryRun = flag.Bool("dry-run", false, "")
	verbose = flag.Bool("verbose", false, "")

	flag.CommandLine.Parse([]string{})

	os.Setenv("DIFY_BASE_URL", "https://env.example.com")
	os.Setenv("DIFY_EMAIL", "env@example.com")
	os.Setenv("DIFY_PASSWORD", "envpassword")
	os.Setenv("DSL_DIRECTORY", "env-dsl")
	os.Setenv("APP_MAP_FILE", "env-map.json")

	config, err = loadConfigAndValidate()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config.DifyBaseURL != "https://env.example.com" {
		t.Errorf("Expected DifyBaseURL to be 'https://env.example.com', got '%s'", config.DifyBaseURL)
	}

	if config.DifyEmail != "env@example.com" {
		t.Errorf("Expected DifyEmail to be 'env@example.com', got '%s'", config.DifyEmail)
	}

	if config.DifyPassword != "envpassword" {
		t.Errorf("Expected DifyPassword to be 'envpassword', got '%s'", config.DifyPassword)
	}
}

func TestPrintInfo(t *testing.T) {
	// This is mostly a visual test, we just check that it doesn't panic
	config := &syncer.Config{
		DifyBaseURL:  "https://test.example.com",
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: "/path/to/dsl",
		AppMapFile:   "/path/to/app_map.json",
		DryRun:       true,
		Verbose:      true,
	}

	// Should not panic
	printInfo(config)
}

func TestPrintStats(t *testing.T) {
	// This is mostly a visual test, we just check that it doesn't panic
	stats := &syncer.SyncStats{
		Total:     10,
		Downloads: 2,
		NoAction:  7,
		Errors:    1,
		StartTime: time.Now().Add(-1 * time.Minute),
		EndTime:   time.Now(),
	}

	// Should not panic
	printStats(stats, 1*time.Minute)
}

// MockSyncer implements the syncer.Syncer interface for testing
type MockSyncer struct {
	stats *syncer.SyncStats
	err   error
}

// LoadAppMap implements the syncer.Syncer interface
func (m *MockSyncer) LoadAppMap() (*syncer.AppMap, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &syncer.AppMap{
		Apps: []syncer.AppMapping{
			{
				Filename: "test.yaml",
				AppID:    "test-app-id",
			},
		},
	}, nil
}

// SyncAll implements the syncer.Syncer interface
func (m *MockSyncer) SyncAll() (*syncer.SyncStats, error) {
	return m.stats, m.err
}

// SyncApp implements the syncer.Syncer interface
func (m *MockSyncer) SyncApp(app syncer.AppMapping) syncer.SyncResult {
	return syncer.SyncResult{
		Filename:  app.Filename,
		AppID:     app.AppID,
		Action:    syncer.ActionNone,
		Success:   true,
		Timestamp: time.Now(),
	}
}

func TestRunSync(t *testing.T) {
	// Save the original factory function
	originalFactory := createSyncer
	defer func() {
		createSyncer = originalFactory
	}()

	// Test successful sync with no errors
	createSyncer = func(config syncer.Config) syncer.Syncer {
		return &MockSyncer{
			stats: &syncer.SyncStats{
				Total:     2,
				Downloads: 0,
				NoAction:  2,
				Errors:    0,
				StartTime: time.Now().Add(-1 * time.Second),
				EndTime:   time.Now(),
			},
			err: nil,
		}
	}

	config := &syncer.Config{
		DifyBaseURL:  "https://test.example.com",
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: "/path/to/dsl",
		AppMapFile:   "/path/to/app_map.json",
	}

	exitCode, err := runSync(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Test sync with stats errors
	createSyncer = func(config syncer.Config) syncer.Syncer {
		return &MockSyncer{
			stats: &syncer.SyncStats{
				Total:     2,
				Downloads: 0,
				NoAction:  0,
				Errors:    2,
				StartTime: time.Now().Add(-1 * time.Second),
				EndTime:   time.Now(),
			},
			err: nil,
		}
	}

	exitCode, err = runSync(config)
	if err != nil {
		t.Errorf("Expected no error (just non-zero exit code), got %v", err)
	}
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}

	// Test sync with error
	createSyncer = func(config syncer.Config) syncer.Syncer {
		return &MockSyncer{
			stats: nil,
			err:   fmt.Errorf("mock error"),
		}
	}

	exitCode, err = runSync(config)
	if err == nil {
		t.Errorf("Expected error")
	}
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
}

// MockSyncerWithInit implements both Syncer and has InitializeAppMap method
type MockSyncerWithInit struct {
	*MockSyncer
	appMap  *syncer.AppMap
	initErr error
}

// InitializeAppMap mocks the DefaultSyncer.InitializeAppMap method
func (m *MockSyncerWithInit) InitializeAppMap() (*syncer.AppMap, error) {
	return m.appMap, m.initErr
}

func TestRunInit(t *testing.T) {
	// Save the original factory function
	originalFactory := createSyncer
	defer func() {
		createSyncer = originalFactory
	}()

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-runInit-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a DSL directory
	dslDir := filepath.Join(tmpDir, "dsl")
	if err := os.MkdirAll(dslDir, 0755); err != nil {
		t.Fatalf("Failed to create DSL directory: %v", err)
	}

	// Create app map file path
	appMapPath := filepath.Join(tmpDir, "app_map.json")

	// Test with successful initialization
	mockSyncer := &MockSyncerWithInit{
		MockSyncer: &MockSyncer{},
		appMap: &syncer.AppMap{
			Apps: []syncer.AppMapping{
				{
					Filename: "test1.yaml",
					AppID:    "test-app-id-1",
				},
				{
					Filename: "test2.yaml",
					AppID:    "test-app-id-2",
				},
			},
		},
	}

	createSyncer = func(config syncer.Config) syncer.Syncer {
		return mockSyncer
	}

	config := &syncer.Config{
		DifyBaseURL:  "https://test.example.com",
		DifyEmail:    "test@example.com",
		DifyPassword: "testpassword",
		DSLDirectory: dslDir,
		AppMapFile:   appMapPath,
	}

	exitCode, err := runInit(config)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Test initialization with error
	mockSyncer.initErr = fmt.Errorf("mock initialization error")

	exitCode, err = runInit(config)
	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}

	// Test when syncer is not DefaultSyncer
	createSyncer = func(config syncer.Config) syncer.Syncer {
		return &MockSyncer{} // This doesn't implement InitializeAppMap
	}

	exitCode, err = runInit(config)
	if err == nil {
		t.Errorf("Expected error about failed conversion, got none")
	}
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
}

// TestMainFunction tests the main function with various commands
func TestMainFunction(t *testing.T) {
	// Save original functions and os.Args
	origArgs := os.Args
	origExit := osExit
	origCreateSyncer := createSyncer

	// Mock exit function
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		// Don't actually exit
	}

	defer func() {
		// Restore original functions and os.Args
		os.Args = origArgs
		osExit = origExit
		createSyncer = origCreateSyncer
		// Clear env vars
		os.Unsetenv("DIFY_BASE_URL")
		os.Unsetenv("DIFY_EMAIL")
		os.Unsetenv("DIFY_PASSWORD")
		os.Unsetenv("DSL_DIRECTORY")
		os.Unsetenv("APP_MAP_FILE")
	}()

	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "difync-test-main-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create DSL directory
	dslDir := filepath.Join(tmpDir, "dsl")
	if err := os.MkdirAll(dslDir, 0755); err != nil {
		t.Fatalf("Failed to create DSL directory: %v", err)
	}

	// Create app map file path and content for sync test
	appMapPath := filepath.Join(tmpDir, "app_map.json")
	appMapContent := `{"apps":[{"filename":"test.yaml","app_id":"test-app-id"}]}`
	if err := os.WriteFile(appMapPath, []byte(appMapContent), 0644); err != nil {
		t.Fatalf("Failed to create app map file: %v", err)
	}

	// Setup test cases
	testCases := []struct {
		name          string
		args          []string
		envVars       map[string]string
		mockSyncer    syncer.Syncer
		expectedCode  int
		shouldRecover bool // Set to true if we expect a panic that should be recovered
	}{
		{
			name: "successful_sync",
			args: []string{"difync"},
			envVars: map[string]string{
				"DIFY_BASE_URL": "https://test.example.com",
				"DIFY_EMAIL":    "test@example.com",
				"DIFY_PASSWORD": "testpassword",
				"DSL_DIRECTORY": dslDir,
				"APP_MAP_FILE":  appMapPath,
			},
			mockSyncer: &MockSyncer{
				stats: &syncer.SyncStats{
					Total:     2,
					Downloads: 0,
					NoAction:  2,
					Errors:    0,
				},
				err: nil,
			},
			expectedCode:  0,
			shouldRecover: false,
		},
		{
			name: "successful_init",
			args: []string{"difync", "init"},
			envVars: map[string]string{
				"DIFY_BASE_URL": "https://test.example.com",
				"DIFY_EMAIL":    "test@example.com",
				"DIFY_PASSWORD": "testpassword",
				"DSL_DIRECTORY": dslDir,
				"APP_MAP_FILE":  appMapPath,
			},
			mockSyncer: &MockSyncerWithInit{
				MockSyncer: &MockSyncer{
					stats: &syncer.SyncStats{},
					err:   nil,
				},
				appMap: &syncer.AppMap{
					Apps: []syncer.AppMapping{
						{
							Filename: "test.yaml",
							AppID:    "test-app-id",
						},
					},
				},
				initErr: nil,
			},
			expectedCode:  0,
			shouldRecover: false,
		},
		{
			name: "invalid_config",
			args: []string{"difync"},
			envVars: map[string]string{
				// Only partial configuration - missing required values
				"DSL_DIRECTORY": dslDir,
				"APP_MAP_FILE":  appMapPath,
			},
			mockSyncer:    nil, // Won't be used due to config error
			expectedCode:  1,
			shouldRecover: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear environment variables first
			os.Unsetenv("DIFY_BASE_URL")
			os.Unsetenv("DIFY_EMAIL")
			os.Unsetenv("DIFY_PASSWORD")
			os.Unsetenv("DSL_DIRECTORY")
			os.Unsetenv("APP_MAP_FILE")

			// Set arguments
			os.Args = tc.args

			// Set environment variables
			for k, v := range tc.envVars {
				os.Setenv(k, v)
			}

			// Set mock syncer if provided
			if tc.mockSyncer != nil {
				createSyncer = func(config syncer.Config) syncer.Syncer {
					return tc.mockSyncer
				}
			} else {
				// Default factory that won't be called due to config errors
				createSyncer = origCreateSyncer
			}

			// For tests that might panic, use a defer/recover
			if tc.shouldRecover {
				defer func() {
					if r := recover(); r != nil {
						// Recovered from panic as expected
						t.Logf("Recovered from expected panic: %v", r)
					}
				}()
			}

			// Call main (this will set exitCode through our mock osExit)
			exitCode = 0 // Reset exitCode
			main()

			// Check exit code
			if exitCode != tc.expectedCode {
				t.Errorf("Expected exit code %d, got %d", tc.expectedCode, exitCode)
			}
		})
	}
}
