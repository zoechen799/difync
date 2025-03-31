package syncer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	DifyBaseURL    string
	DifyToken      string
	DSLDirectory   string
	AppMapFile     string
	DryRun         bool
	ForceDirection string // "upload", "download", or "" (bidirectional)
	Verbose        bool
}

// DefaultSyncer handles the synchronization between local DSL files and Dify
type DefaultSyncer struct {
	config Config
	client *api.Client
}

// NewSyncer creates a new syncer with the given configuration
func NewSyncer(config Config) Syncer {
	return &DefaultSyncer{
		config: config,
		client: api.NewClient(config.DifyBaseURL, config.DifyToken),
	}
}

// LoadAppMap loads the app map from the app map file
func (s *DefaultSyncer) LoadAppMap() (*AppMap, error) {
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

	for _, app := range appMap.Apps {
		result := s.SyncApp(app)

		switch result.Action {
		case ActionUpload:
			stats.Uploads++
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

	stats.EndTime = time.Now()
	return stats, nil
}

// SyncApp synchronizes a single app
func (s *DefaultSyncer) SyncApp(app AppMapping) SyncResult {
	result := SyncResult{
		Filename:  app.Filename,
		AppID:     app.AppID,
		Timestamp: time.Now(),
	}

	// Get local file modification time
	localPath := filepath.Join(s.config.DSLDirectory, app.Filename)
	localInfo, err := os.Stat(localPath)
	if err != nil {
		result.Action = ActionError
		result.Error = fmt.Errorf("failed to stat local file: %w", err)
		return result
	}
	localModTime := localInfo.ModTime()

	// Get remote app info
	appInfo, err := s.client.GetAppInfo(app.AppID)
	if err != nil {
		result.Action = ActionError
		result.Error = fmt.Errorf("failed to get app info: %w", err)
		return result
	}
	remoteModTime := appInfo.UpdatedAt

	// Determine sync direction based on modification times and force direction
	if s.config.ForceDirection == "upload" {
		return s.uploadToRemote(app, localPath)
	} else if s.config.ForceDirection == "download" {
		return s.downloadFromRemote(app, localPath)
	}

	// Bidirectional sync based on modification time
	if localModTime.After(remoteModTime) {
		return s.uploadToRemote(app, localPath)
	} else if remoteModTime.After(localModTime) {
		return s.downloadFromRemote(app, localPath)
	}

	// Files are in sync
	result.Action = ActionNone
	result.Success = true
	return result
}

// uploadToRemote uploads the local DSL file to Dify
func (s *DefaultSyncer) uploadToRemote(app AppMapping, localPath string) SyncResult {
	result := SyncResult{
		Filename:  app.Filename,
		AppID:     app.AppID,
		Action:    ActionUpload,
		Timestamp: time.Now(),
	}

	// Read local DSL file
	dsl, err := os.ReadFile(localPath)
	if err != nil {
		result.Error = fmt.Errorf("failed to read local DSL file: %w", err)
		return result
	}

	// If dry run, just return success
	if s.config.DryRun {
		result.Success = true
		return result
	}

	// Update DSL in Dify
	if err := s.client.UpdateDSL(app.AppID, dsl); err != nil {
		result.Error = fmt.Errorf("failed to update DSL in Dify: %w", err)
		return result
	}

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
