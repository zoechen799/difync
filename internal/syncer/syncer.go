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
	DifyBaseURL    string
	DifyEmail      string
	DifyPassword   string
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
	client := api.NewClient(config.DifyBaseURL)

	// ログインしてトークンを取得
	if err := client.Login(config.DifyEmail, config.DifyPassword); err != nil {
		// エラーが発生した場合は、ログにエラーを記録する
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
		// ファイルが存在しない場合は、dry-runモードに関わらず初期化を促す
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

	// ファイル名の重複を避けるためのマップ
	usedFilenames := make(map[string]bool)

	// For each app, add an entry to the app map
	for _, app := range appList {
		// Create a safe filename from app name
		// 日本語などの文字も保持する
		safeName := s.sanitizeFilename(app.Name)
		fmt.Printf("Debug - sanitizeFilename(%q) = %q\n", app.Name, safeName)
		filename := safeName + ".yaml"

		// 同名ファイルの重複を避ける
		// ファイルシステム上に存在するかチェック
		fileExists := s.fileExists(filepath.Join(s.config.DSLDirectory, filename))
		// すでに追加済みのファイル名かチェック
		filenameUsed := usedFilenames[filename]

		counter := 1
		baseName := safeName

		// ファイル名が重複する限りループ
		for fileExists || filenameUsed {
			fmt.Printf("Debug - File exists or already used: %s, incrementing counter to %d\n", filename, counter)
			filename = fmt.Sprintf("%s_%d.yaml", baseName, counter)
			fileExists = s.fileExists(filepath.Join(s.config.DSLDirectory, filename))
			filenameUsed = usedFilenames[filename]
			counter++
		}

		fmt.Printf("Debug - Final filename for app %q (ID: %s): %s\n", app.Name, app.ID, filename)

		// 使用するファイル名を記録
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
	// 結果の文字列
	var result strings.Builder

	// ファイル名に使用できない文字を置き換える
	// Windows, macOS, Linuxで共通して使用できない文字: / \ : * ? " < > |
	invalidChars := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|'}

	// スペースはアンダースコアに変換
	for _, r := range name {
		if unicode.IsSpace(r) {
			result.WriteRune('_')
		} else {
			// ファイル名に使用できない文字をチェック
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

	// 結果が空の場合はデフォルト名を使用
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
	remoteModTime := time.Now() // デフォルト値として現在時刻を設定

	switch v := appInfo.UpdatedAt.(type) {
	case string:
		// 文字列の場合：文字列タイムスタンプを解析
		if v != "" {
			// RFC3339形式を試す (2023-01-02T15:04:05Z)
			parsedTime, err := time.Parse(time.RFC3339, v)
			if err == nil {
				remoteModTime = parsedTime
			} else {
				// 他の形式を試す
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
		}
	case float64:
		// 数値の場合：UNIXタイムスタンプとして解釈（秒単位）
		remoteModTime = time.Unix(int64(v), 0)
		fmt.Printf("Debug - Converted float64 timestamp %v to time: %v\n", v, remoteModTime)
	case int:
		// 整数の場合：UNIXタイムスタンプとして解釈（秒単位）
		remoteModTime = time.Unix(int64(v), 0)
		fmt.Printf("Debug - Converted int timestamp %v to time: %v\n", v, remoteModTime)
	case int64:
		// 64ビット整数の場合：UNIXタイムスタンプとして解釈
		remoteModTime = time.Unix(v, 0)
		fmt.Printf("Debug - Converted int64 timestamp %v to time: %v\n", v, remoteModTime)
	case json.Number:
		// json.Number型の場合
		if i, err := v.Int64(); err == nil {
			remoteModTime = time.Unix(i, 0)
			fmt.Printf("Debug - Converted json.Number timestamp %v to time: %v\n", v, remoteModTime)
		}
	default:
		fmt.Printf("Debug - Unknown type for UpdatedAt: %T value: %v\n", appInfo.UpdatedAt, appInfo.UpdatedAt)
	}

	fmt.Printf("Debug - Local mod time: %v, Remote mod time: %v\n", localModTime, remoteModTime)

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
