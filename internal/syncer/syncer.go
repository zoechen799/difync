package syncer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/pepabo/difync/internal/api"
)

// Syncer defines the interface for syncing between local DSL files and Dify
type Syncer interface {
	LoadAppMap() (*AppMap, error)
	SyncAll() (*SyncStats, error)
	SyncApp(app AppMapping) SyncResult
}

// Config represents the configuration for the syncer
type Config struct {
	DifyBaseURL  string
	DifyEmail    string
	DifyPassword string
	DSLDirectory string
	AppMapFile   string
	DryRun       bool
	Verbose      bool
}

// DefaultSyncer handles the synchronization between local DSL files and Dify
type DefaultSyncer struct {
	config Config
	client *api.Client
}

// NewSyncer creates a new syncer with the given configuration
func NewSyncer(config Config) Syncer {
	client := api.NewClient(config.DifyBaseURL)

	// Login to get token
	if err := client.Login(config.DifyEmail, config.DifyPassword); err != nil {
		// Log the error if login fails
		fmt.Printf("Failed to login to Dify API: %v\n", err)
	}

	return &DefaultSyncer{
		config: config,
		client: client,
	}
}

// LoadAppMap loads the app map from the app map file
func (s *DefaultSyncer) LoadAppMap() (*AppMap, error) {
	// Check if app map file exists
	_, err := os.Stat(s.config.AppMapFile)
	if os.IsNotExist(err) {
		// If the file doesn't exist, prompt for initialization regardless of dry-run mode
		return nil, fmt.Errorf("app map file not found at %s. Please run 'difync init' first to initialize the app map", s.config.AppMapFile)
	}

	file, err := os.Open(s.config.AppMapFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open app map file: %w", err)
	}
	defer file.Close()

	var appMap AppMap
	if err := json.NewDecoder(file).Decode(&appMap); err != nil {
		return nil, fmt.Errorf("failed to decode app map: %w", err)
	}

	return &appMap, nil
}

// InitializeAppMap creates a new app map file by fetching app list from Dify API
func (s *DefaultSyncer) InitializeAppMap() (*AppMap, error) {
	// Fetch application list from API
	appList, err := s.client.GetAppList()
	if err != nil {
		return nil, fmt.Errorf("failed to get app list from API: %w", err)
	}

	if len(appList) == 0 {
		return nil, fmt.Errorf("no applications found in Dify account")
	}

	// Create DSL directory if it doesn't exist
	if err := os.MkdirAll(s.config.DSLDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create DSL directory: %w", err)
	}

	// Create app map
	appMap := &AppMap{
		Apps: make([]AppMapping, 0, len(appList)),
	}

	// Map to track used filenames to avoid duplicates
	usedFilenames := make(map[string]bool)

	// For each app, add an entry to the app map
	for _, app := range appList {
		// Create a safe filename from app name
		// Preserve non-ASCII characters like Japanese
		safeName := s.sanitizeFilename(app.Name)
		fmt.Printf("Debug - sanitizeFilename(%q) = %q\n", app.Name, safeName)
		filename := safeName + ".yaml"

		// Avoid duplicate filenames
		// Check if file exists in filesystem
		fileExists := s.fileExists(filepath.Join(s.config.DSLDirectory, filename))
		// Check if filename is already used in the map
		filenameUsed := usedFilenames[filename]

		counter := 1
		baseName := safeName

		// Loop until a unique filename is found
		for fileExists || filenameUsed {
			fmt.Printf("Debug - File exists or already used: %s, incrementing counter to %d\n", filename, counter)
			filename = fmt.Sprintf("%s_%d.yaml", baseName, counter)
			fileExists = s.fileExists(filepath.Join(s.config.DSLDirectory, filename))
			filenameUsed = usedFilenames[filename]
			counter++
		}

		fmt.Printf("Debug - Final filename for app %q (ID: %s): %s\n", app.Name, app.ID, filename)

		// Record the filename as used
		usedFilenames[filename] = true

		appMap.Apps = append(appMap.Apps, AppMapping{
			Filename: filename,
			AppID:    app.ID,
		})

		// Also download the DSL for this app if it doesn't exist yet
		localPath := filepath.Join(s.config.DSLDirectory, filename)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			if s.config.Verbose {
				fmt.Printf("Downloading initial DSL for %s to %s\n", app.Name, localPath)
			}

			dsl, err := s.client.GetDSL(app.ID)
			if err != nil {
				fmt.Printf("Warning: Failed to download DSL for %s: %v\n", app.Name, err)
				continue
			}

			if !s.config.DryRun {
				if err := os.WriteFile(localPath, dsl, 0644); err != nil {
					fmt.Printf("Warning: Failed to write DSL file for %s: %v\n", app.Name, err)
				}
			}
		}
	}

	// Write the app map to file
	if !s.config.DryRun {
		appMapDir := filepath.Dir(s.config.AppMapFile)
		if err := os.MkdirAll(appMapDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for app map file: %w", err)
		}

		file, err := os.Create(s.config.AppMapFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create app map file: %w", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(appMap); err != nil {
			return nil, fmt.Errorf("failed to write app map file: %w", err)
		}

		fmt.Printf("Created new app map file at %s with %d applications\n", s.config.AppMapFile, len(appMap.Apps))
	} else {
		fmt.Printf("Dry run: Would create app map file at %s with %d applications\n", s.config.AppMapFile, len(appMap.Apps))
	}

	return appMap, nil
}

// fileExists checks if a file exists
func (s *DefaultSyncer) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// sanitizeFilename creates a safe filename from an app name
func (s *DefaultSyncer) sanitizeFilename(name string) string {
	// Result string
	var result strings.Builder

	// Replace characters not allowed in filenames
	// Characters invalid across Windows, macOS, Linux: / \ : * ? " < > |
	invalidChars := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|'}

	// Convert spaces to underscores
	for _, r := range name {
		if unicode.IsSpace(r) {
			result.WriteRune('_')
		} else {
			// Check for invalid characters
			invalid := false
			for _, ic := range invalidChars {
				if r == ic {
					invalid = true
					break
				}
			}

			if !invalid {
				result.WriteRune(r)
			}
		}
	}

	// Use default name if result is empty
	if result.Len() == 0 {
		return "app"
	}

	fmt.Printf("Debug - sanitizeFilename internal: %q -> %q\n", name, result.String())
	return result.String()
}

// SyncAll synchronizes all apps in the app map
func (s *DefaultSyncer) SyncAll() (*SyncStats, error) {
	appMap, err := s.LoadAppMap()
	if err != nil {
		return nil, err
	}

	stats := &SyncStats{
		Total:     len(appMap.Apps),
		StartTime: time.Now(),
	}

	// First, check for remote apps that have been deleted
	deletedApps := []AppMapping{}

	for _, app := range appMap.Apps {
		// Check if the app still exists in remote
		exists, err := s.client.DoesDSLExist(app.AppID)
		if err != nil {
			fmt.Printf("Warning: Failed to check if app %s exists: %v\n", app.AppID, err)
			continue
		}

		if !exists {
			// App has been deleted remotely
			deletedApps = append(deletedApps, app)
			if s.config.Verbose {
				fmt.Printf("App %s (ID: %s) has been deleted remotely\n", app.Filename, app.AppID)
			}

			// Delete local file if not in dry run mode
			if !s.config.DryRun {
				localPath := filepath.Join(s.config.DSLDirectory, app.Filename)
				if err := os.Remove(localPath); err != nil {
					fmt.Printf("Warning: Failed to delete local file %s: %v\n", localPath, err)
				} else if s.config.Verbose {
					fmt.Printf("Deleted local file %s\n", localPath)
				}
			}

			// Count as download since we're reflecting remote state
			stats.Downloads++
			continue
		}

		// Process existing apps
		result := s.SyncApp(app)

		switch result.Action {
		case ActionDownload:
			stats.Downloads++
		case ActionNone:
			stats.NoAction++
		case ActionError:
			stats.Errors++
		}

		if s.config.Verbose {
			fmt.Printf("Synced %s (app_id: %s): %s\n", app.Filename, app.AppID, result.Action)
			if result.Error != nil {
				fmt.Printf("  Error: %v\n", result.Error)
			}
		}
	}

	// Update app map if apps were deleted
	if len(deletedApps) > 0 && !s.config.DryRun {
		// Create new app map without deleted apps
		newApps := make([]AppMapping, 0, len(appMap.Apps)-len(deletedApps))
		for _, app := range appMap.Apps {
			isDeleted := false
			for _, deletedApp := range deletedApps {
				if app.AppID == deletedApp.AppID {
					isDeleted = true
					break
				}
			}
			if !isDeleted {
				newApps = append(newApps, app)
			}
		}
		appMap.Apps = newApps

		// Write updated app map to file
		appMapFile, err := os.Create(s.config.AppMapFile)
		if err != nil {
			return nil, fmt.Errorf("failed to update app map file: %w", err)
		}
		defer appMapFile.Close()

		encoder := json.NewEncoder(appMapFile)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(appMap); err != nil {
			return nil, fmt.Errorf("failed to write updated app map: %w", err)
		}

		if s.config.Verbose {
			fmt.Printf("Updated app map file at %s, removed %d deleted apps\n", s.config.AppMapFile, len(deletedApps))
		}
	}

	stats.EndTime = time.Now()
	return stats, nil
}

// SyncApp synchronizes a single app
func (s *DefaultSyncer) SyncApp(app AppMapping) SyncResult {
	result := SyncResult{
		Filename:  app.Filename,
		AppID:     app.AppID,
		Action:    ActionNone,
		Timestamp: time.Now(),
	}

	// Get local file modification time
	localPath := filepath.Join(s.config.DSLDirectory, app.Filename)
	localInfo, err := os.Stat(localPath)
	if err != nil {
		result.Action = ActionError
		result.Error = fmt.Errorf("failed to stat local file: %w", err)
		if s.config.Verbose {
			fmt.Printf("Error: %v\n", result.Error)
		}
		return result
	}
	localModTime := localInfo.ModTime()

	// Check if app still exists remotely
	exists, err := s.client.DoesDSLExist(app.AppID)
	if err != nil {
		result.Action = ActionError
		result.Error = fmt.Errorf("failed to check if app exists: %w", err)
		if s.config.Verbose {
			fmt.Printf("Error checking app %s (%s): %v\n", app.AppID, app.Filename, err)
		}
		return result
	}

	if !exists {
		// App has been deleted remotely
		if s.config.Verbose {
			fmt.Printf("App %s (ID: %s) no longer exists remotely\n", app.Filename, app.AppID)
		}

		// We'll handle the deletion in SyncAll
		result.Action = ActionNone
		result.Success = true
		return result
	}

	// Get remote app info
	appInfo, err := s.client.GetAppInfo(app.AppID)
	if err != nil {
		result.Action = ActionError
		result.Error = fmt.Errorf("failed to get app info: %w", err)
		if s.config.Verbose {
			fmt.Printf("Error accessing app %s (%s): %v\n", app.AppID, app.Filename, err)
		}
		return result
	}

	fmt.Printf("Debug - App Info for %s: %+v\n", app.AppID, appInfo)

	// Convert interface{} updated_at to time.Time
	var remoteModTime time.Time
	var useLocalTime bool = false

	if appInfo.UpdatedAt == nil {
		// If UpdatedAt is nil, use a time in the past to ensure the local file is considered newer
		fmt.Printf("Debug - UpdatedAt is nil, using past timestamp to prioritize local file\n")
		// Use Unix epoch start as the remote time (1970-01-01) to ensure local is newer
		remoteModTime = time.Unix(0, 0)
		useLocalTime = true
	} else {
		switch v := appInfo.UpdatedAt.(type) {
		case string:
			// For string type: parse the timestamp string
			if v != "" {
				// Try RFC3339 format (2023-01-02T15:04:05Z)
				parsedTime, err := time.Parse(time.RFC3339, v)
				if err == nil {
					remoteModTime = parsedTime
				} else {
					// Try other formats
					layouts := []string{
						"2006-01-02 15:04:05",
						"2006-01-02T15:04:05",
						"2006/01/02 15:04:05",
						time.RFC1123,
						time.RFC1123Z,
					}

					for _, layout := range layouts {
						parsedTime, err := time.Parse(layout, v)
						if err == nil {
							remoteModTime = parsedTime
							break
						}
					}
				}
			} else {
				// Empty string, treat as nil case
				fmt.Printf("Debug - UpdatedAt is empty string, using past timestamp to prioritize local file\n")
				remoteModTime = time.Unix(0, 0)
				useLocalTime = true
			}
		case float64:
			// For numeric type: interpret as UNIX timestamp (seconds)
			remoteModTime = time.Unix(int64(v), 0)
			fmt.Printf("Debug - Converted float64 timestamp %v to time: %v\n", v, remoteModTime)
		case int:
			// For integer type: interpret as UNIX timestamp (seconds)
			remoteModTime = time.Unix(int64(v), 0)
			fmt.Printf("Debug - Converted int timestamp %v to time: %v\n", v, remoteModTime)
		case int64:
			// For 64-bit integer: interpret as UNIX timestamp
			remoteModTime = time.Unix(v, 0)
			fmt.Printf("Debug - Converted int64 timestamp %v to time: %v\n", v, remoteModTime)
		case json.Number:
			// For json.Number type
			if i, err := v.Int64(); err == nil {
				remoteModTime = time.Unix(i, 0)
				fmt.Printf("Debug - Converted json.Number timestamp %v to time: %v\n", v, remoteModTime)
			} else {
				// If conversion fails, treat as nil case
				fmt.Printf("Debug - Could not convert json.Number %v to timestamp, using past timestamp\n", v)
				remoteModTime = time.Unix(0, 0)
				useLocalTime = true
			}
		default:
			fmt.Printf("Debug - Unknown type for UpdatedAt: %T value: %v, using past timestamp\n", appInfo.UpdatedAt, appInfo.UpdatedAt)
			remoteModTime = time.Unix(0, 0)
			useLocalTime = true
		}
	}

	fmt.Printf("Debug - Local mod time: %v, Remote mod time: %v\n", localModTime, remoteModTime)

	// If UpdatedAt was nil or couldn't be parsed, don't sync
	if useLocalTime {
		fmt.Printf("Debug - No valid remote timestamp found, skipping sync\n")
		result.Action = ActionNone
		result.Success = true
		return result
	}

	// Only download if remote is newer
	if remoteModTime.After(localModTime) {
		return s.downloadFromRemote(app, localPath)
	}

	// Files are in sync
	result.Action = ActionNone
	result.Success = true
	return result
}

// downloadFromRemote downloads the DSL from Dify to the local file
func (s *DefaultSyncer) downloadFromRemote(app AppMapping, localPath string) SyncResult {
	result := SyncResult{
		Filename:  app.Filename,
		AppID:     app.AppID,
		Action:    ActionDownload,
		Timestamp: time.Now(),
	}

	// Get DSL from Dify
	dsl, err := s.client.GetDSL(app.AppID)
	if err != nil {
		result.Error = fmt.Errorf("failed to get DSL from Dify: %w", err)
		return result
	}

	// If dry run, just return success
	if s.config.DryRun {
		result.Success = true
		return result
	}

	// Write DSL to local file
	if err := os.WriteFile(localPath, dsl, 0644); err != nil {
		result.Error = fmt.Errorf("failed to write DSL to local file: %w", err)
		return result
	}

	result.Success = true
	return result
}
