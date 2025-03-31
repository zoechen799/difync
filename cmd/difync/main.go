// Package main is the entry point for the difync CLI application
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/pepabo/difync/internal/syncer"
)

// getEnvWithDefault gets environment variable or returns default if not set
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Command-line flags
var (
	difyBaseURL    = flag.String("base-url", "", "Dify API base URL (overrides env: DIFY_BASE_URL)")
	difyToken      = flag.String("token", "", "Dify API token (overrides env: DIFY_API_TOKEN)")
	dslDir         = flag.String("dsl-dir", "", "Directory containing DSL files (overrides env: DSL_DIRECTORY, default: dsl)")
	appMapFile     = flag.String("app-map", "", "Path to app mapping file (overrides env: APP_MAP_FILE, default: app_map.json)")
	dryRun         = flag.Bool("dry-run", false, "Perform a dry run without making any changes")
	forceDirection = flag.String("force", "", "Force sync direction: 'upload', 'download', or empty for bidirectional")
	verbose        = flag.Bool("verbose", false, "Enable verbose output")
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	flag.Parse()

	// Get values from environment if not set via flags
	baseURL := *difyBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("DIFY_BASE_URL")
	}

	token := *difyToken
	if token == "" {
		token = os.Getenv("DIFY_API_TOKEN")
	}

	// Get DSL directory from flags or environment with default
	dslDirectory := *dslDir
	if dslDirectory == "" {
		dslDirectory = getEnvWithDefault("DSL_DIRECTORY", "dsl")
	}

	// Get app map file from flags or environment with default
	appMap := *appMapFile
	if appMap == "" {
		appMap = getEnvWithDefault("APP_MAP_FILE", "app_map.json")
	}

	// Validate required parameters
	if baseURL == "" {
		fmt.Println("Error: Dify base URL is required. Set with --base-url or DIFY_BASE_URL env var.")
		os.Exit(1)
	}

	if token == "" {
		fmt.Println("Error: Dify API token is required. Set with --token or DIFY_API_TOKEN env var.")
		os.Exit(1)
	}

	// Resolve DSL directory path
	dslDirPath, err := filepath.Abs(dslDirectory)
	if err != nil {
		fmt.Printf("Error: Failed to resolve DSL directory path: %v\n", err)
		os.Exit(1)
	}

	// Resolve app map file path
	appMapPath, err := filepath.Abs(appMap)
	if err != nil {
		fmt.Printf("Error: Failed to resolve app map file path: %v\n", err)
		os.Exit(1)
	}

	// Create syncer config
	config := syncer.Config{
		DifyBaseURL:    baseURL,
		DifyToken:      token,
		DSLDirectory:   dslDirPath,
		AppMapFile:     appMapPath,
		DryRun:         *dryRun,
		ForceDirection: *forceDirection,
		Verbose:        *verbose,
	}

	// Validate force direction if provided
	if *forceDirection != "" && *forceDirection != "upload" && *forceDirection != "download" {
		fmt.Printf("Error: Invalid force direction '%s'. Must be 'upload', 'download', or empty.\n", *forceDirection)
		os.Exit(1)
	}

	// Create and run syncer
	syncr := syncer.NewSyncer(config)

	// Print info
	fmt.Println("Difync - Dify.AI DSL Synchronizer")
	fmt.Println("----------------------------")
	fmt.Printf("DSL Directory: %s\n", dslDirPath)
	fmt.Printf("App Map File: %s\n", appMapPath)
	if *dryRun {
		fmt.Println("Mode: DRY RUN (no changes will be made)")
	} else if *forceDirection != "" {
		fmt.Printf("Mode: Force %s\n", *forceDirection)
	} else {
		fmt.Println("Mode: Bidirectional sync")
	}
	fmt.Println()

	// Start sync
	fmt.Println("Starting sync...")
	startTime := time.Now()

	stats, err := syncr.SyncAll()
	if err != nil {
		fmt.Printf("Error during sync: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\nSync Summary:")
	fmt.Printf("Total apps: %d\n", stats.Total)
	fmt.Printf("Uploads: %d\n", stats.Uploads)
	fmt.Printf("Downloads: %d\n", stats.Downloads)
	fmt.Printf("No action (in sync): %d\n", stats.NoAction)
	fmt.Printf("Errors: %d\n", stats.Errors)
	fmt.Printf("Duration: %v\n", duration)

	// Exit with non-zero status if there were errors
	if stats.Errors > 0 {
		os.Exit(1)
	}
}
