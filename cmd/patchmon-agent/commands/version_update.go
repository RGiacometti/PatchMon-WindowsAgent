package commands

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"patchmon-agent/internal/config"
	"patchmon-agent/internal/version"

	"github.com/spf13/cobra"
)

const (
	serverTimeout       = 30 * time.Second
	versionCheckTimeout = 10 * time.Second // Shorter timeout for version checks
)

type ServerVersionResponse struct {
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
	Size         int64  `json:"size"`
	Hash         string `json:"hash"`
	DownloadURL  string `json:"downloadUrl"`
	BinaryData   []byte `json:"-"` // Binary data (not serialized to JSON)
}

type ServerVersionInfo struct {
	CurrentVersion           string   `json:"currentVersion"`
	LatestVersion            string   `json:"latestVersion"`
	HasUpdate                bool     `json:"hasUpdate"`
	AutoUpdateDisabled       bool     `json:"autoUpdateDisabled"`
	AutoUpdateDisabledReason string   `json:"autoUpdateDisabledReason"`
	LastChecked              string   `json:"lastChecked"`
	SupportedArchitectures   []string `json:"supportedArchitectures"`
}

// checkVersionCmd represents the check-version command
var checkVersionCmd = &cobra.Command{
	Use:   "check-version",
	Short: "Check for agent updates",
	Long:  "Check if there are any updates available for the PatchMon agent.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkAdmin(); err != nil {
			return err
		}

		return checkVersion()
	},
}

// updateAgentCmd represents the update-agent command
var updateAgentCmd = &cobra.Command{
	Use:   "update-agent",
	Short: "Update agent to latest version",
	Long:  "Download and install the latest version of the PatchMon agent.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkAdmin(); err != nil {
			return err
		}

		return updateAgent()
	},
}

func checkVersion() error {
	logger.Info("Checking for agent updates...")

	versionInfo, err := getServerVersionInfo()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	currentVersion := strings.TrimPrefix(version.Version, "v")
	latestVersion := strings.TrimPrefix(versionInfo.LatestVersion, "v")

	if versionInfo.HasUpdate {
		logger.Info("Agent update available!")
		fmt.Printf("  Current version: %s\n", currentVersion)
		fmt.Printf("  Latest version: %s\n", latestVersion)
		fmt.Printf("\nTo update, run: patchmon-agent update-agent\n")
	} else if versionInfo.AutoUpdateDisabled && latestVersion != currentVersion {
		logger.WithFields(map[string]interface{}{
			"current": currentVersion,
			"latest":  latestVersion,
			"reason":  versionInfo.AutoUpdateDisabledReason,
		}).Info("New update available but auto-update is disabled")
		fmt.Printf("Current version: %s\n", currentVersion)
		fmt.Printf("Latest version: %s\n", latestVersion)
		fmt.Printf("Status: %s\n", versionInfo.AutoUpdateDisabledReason)
		fmt.Printf("\nTo update manually, run: patchmon-agent update-agent\n")
	} else {
		logger.WithField("version", currentVersion).Info("Agent is up to date")
		fmt.Printf("Agent is up to date (version %s)\n", currentVersion)
	}

	return nil
}

func updateAgent() error {
	logger.Info("Updating agent...")

	// Check if we recently updated to prevent update loops
	if err := checkRecentUpdate(); err != nil {
		logger.WithError(err).Warn("Recent update detected, skipping to prevent update loop")
		return fmt.Errorf("update skipped: %w", err)
	}

	// Get current executable path
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	resolvedPath, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		logger.WithError(err).WithField("path", executablePath).Warn("Could not resolve symlinks, using original path")
	} else if resolvedPath != executablePath {
		logger.WithField("original", executablePath).WithField("resolved", resolvedPath).Debug("Resolved executable symlink")
		executablePath = resolvedPath
	}

	// Get current version for comparison
	currentVersion := strings.TrimPrefix(version.Version, "v")

	// First, check server version info to see if update is needed
	logger.Debug("Checking server for latest version...")
	versionInfo, err := getServerVersionInfo()
	if err != nil {
		logger.WithError(err).Warn("Failed to get version info, proceeding with update anyway")
	} else {
		latestVersion := strings.TrimPrefix(versionInfo.LatestVersion, "v")
		logger.WithField("current", currentVersion).WithField("latest", latestVersion).Debug("Version check")

		// Check if update is actually needed
		if currentVersion == latestVersion && !versionInfo.HasUpdate {
			logger.WithField("version", currentVersion).Info("Agent is already at the latest version, skipping update")
			return nil
		}
	}

	// Get latest binary info from server
	binaryInfo, err := getLatestBinaryFromServer()
	if err != nil {
		return fmt.Errorf("failed to get latest binary information: %w", err)
	}

	newAgentData := binaryInfo.BinaryData
	if len(newAgentData) == 0 {
		return fmt.Errorf("no binary data received from server")
	}

	// Get the new version from server version info
	newVersion := currentVersion // Default to current if we can't determine
	if versionInfo != nil && versionInfo.LatestVersion != "" {
		newVersion = strings.TrimPrefix(versionInfo.LatestVersion, "v")
	}

	logger.WithField("current", currentVersion).WithField("new", newVersion).Info("Proceeding with update")
	logger.Info("Using downloaded agent binary...")

	// Clean up old backups before creating new one (keep only last 3)
	cleanupOldBackups(executablePath)

	// Create backup of current executable
	backupPath := fmt.Sprintf("%s.backup.%s", executablePath, time.Now().Format("20060102_150405"))
	if err := copyFile(executablePath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	logger.WithField("path", backupPath).Info("Backup saved")

	// Write new version to temporary file
	tempPath := executablePath + ".new"
	if err := os.WriteFile(tempPath, newAgentData, 0755); err != nil {
		return fmt.Errorf("failed to write new agent: %w", err)
	}

	// Verify the new executable works and check its version
	logger.Debug("Validating new executable...")
	testCmd := exec.Command(tempPath, "check-version")
	testCmd.Env = os.Environ()
	if err := testCmd.Run(); err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil {
			logger.WithError(removeErr).Warn("Failed to remove temporary file after validation failure")
		}
		return fmt.Errorf("new agent executable is invalid: %w", err)
	}
	logger.Debug("New executable validation passed")

	// Verify the downloaded binary version matches expected version
	logger.Debug("Verifying downloaded binary version...")
	versionCmd := exec.Command(tempPath, "version")
	versionCmd.Env = os.Environ()
	versionOutput, err := versionCmd.Output()
	if err == nil {
		versionStr := strings.TrimSpace(string(versionOutput))
		versionStr = strings.TrimPrefix(versionStr, "PatchMon Agent v")
		versionStr = strings.TrimPrefix(versionStr, "v")
		versionStr = strings.TrimSpace(versionStr)

		if versionStr != "" && versionStr != newVersion {
			logger.WithFields(map[string]interface{}{
				"expected": newVersion,
				"actual":   versionStr,
			}).Warn("Downloaded binary version mismatch - this may indicate server issue, but proceeding")
		} else if versionStr == newVersion {
			logger.WithField("version", versionStr).Debug("Downloaded binary version verified")
		}
	} else {
		logger.WithError(err).Debug("Could not verify binary version (non-critical)")
	}

	// Replace current executable
	// On Windows, we cannot rename over a running executable directly.
	// Instead, rename the current exe to .old, then rename .new to the target.
	logger.Debug("Replacing executable...")
	oldPath := executablePath + ".old"
	// Remove any previous .old file
	_ = os.Remove(oldPath)

	if err := os.Rename(executablePath, oldPath); err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil {
			logger.WithError(removeErr).Warn("Failed to remove temporary file after rename failure")
		}
		return fmt.Errorf("failed to move current executable aside: %w", err)
	}

	if err := os.Rename(tempPath, executablePath); err != nil {
		// Try to restore the old executable
		_ = os.Rename(oldPath, executablePath)
		return fmt.Errorf("failed to install new executable: %w", err)
	}

	// Clean up the .old file (may fail if still in use, that's OK)
	_ = os.Remove(oldPath)

	logger.WithField("version", newVersion).Info("Agent updated successfully")

	// Mark that we just updated to prevent immediate re-update loops
	markRecentUpdate()

	// On Windows, we can't restart ourselves easily like on Linux with systemd.
	// Just inform the user to restart manually or via Task Scheduler.
	logger.Info("Agent binary has been updated. Please restart the agent to use the new version.")
	fmt.Printf("Agent updated to version %s. Please restart the agent.\n", newVersion)

	return nil
}

// getServerVersionInfo fetches version information from the PatchMon server
func getServerVersionInfo() (*ServerVersionInfo, error) {
	cfgManager := config.New()
	if err := cfgManager.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	// Load credentials for API authentication
	if err := cfgManager.LoadCredentials(); err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	credentials := cfgManager.GetCredentials()

	architecture := getArchitecture()
	currentVersion := strings.TrimPrefix(version.Version, "v")
	url := fmt.Sprintf("%s/api/v1/hosts/agent/version?arch=%s&type=go&currentVersion=%s", cfg.PatchmonServer, architecture, currentVersion)

	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", fmt.Sprintf("patchmon-agent/%s", version.Version))
	req.Header.Set("X-API-ID", credentials.APIID)
	req.Header.Set("X-API-KEY", credentials.APIKey)

	// Create HTTP client with proper timeouts
	httpClient := &http.Client{
		Timeout: versionCheckTimeout,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}

	// Configure for insecure SSL if needed
	if cfg.SkipSSLVerify {
		httpClient.Transport = &http.Transport{
			ResponseHeaderTimeout: 5 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Debug("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var versionInfo ServerVersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("failed to decode version info: %w", err)
	}

	return &versionInfo, nil
}

// getLatestBinaryFromServer fetches the latest binary information from the PatchMon server
func getLatestBinaryFromServer() (*ServerVersionResponse, error) {
	cfgManager := config.New()
	if err := cfgManager.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg := cfgManager.GetConfig()

	// Load credentials for API authentication
	if err := cfgManager.LoadCredentials(); err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}
	credentials := cfgManager.GetCredentials()

	architecture := getArchitecture()
	url := fmt.Sprintf("%s/api/v1/hosts/agent/download?arch=%s", cfg.PatchmonServer, architecture)

	ctx, cancel := context.WithTimeout(context.Background(), serverTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", fmt.Sprintf("patchmon-agent/%s", version.Version))
	req.Header.Set("X-API-ID", credentials.APIID)
	req.Header.Set("X-API-KEY", credentials.APIKey)

	// Configure HTTP client for insecure SSL if needed
	httpClient := http.DefaultClient
	if cfg.SkipSSLVerify {
		logger.Warn("⚠️  SSL certificate verification is disabled for binary download")
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithError(closeErr).Debug("Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Read the binary data
	binaryData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read binary data: %w", err)
	}

	// Calculate hash
	hash := fmt.Sprintf("%x", sha256.Sum256(binaryData))

	return &ServerVersionResponse{
		Version:      version.Version,
		Architecture: architecture,
		Size:         int64(len(binaryData)),
		Hash:         hash,
		DownloadURL:  url,
		BinaryData:   binaryData,
	}, nil
}

// getArchitecture returns the architecture string for the current platform
func getArchitecture() string {
	return runtime.GOARCH
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0755)
}

// cleanupOldBackups removes old backup files, keeping only the last 3
func cleanupOldBackups(executablePath string) {
	// Find all backup files
	backupDir := filepath.Dir(executablePath)
	backupBase := filepath.Base(executablePath)

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		logger.WithError(err).Debug("Could not read directory to clean up backups")
		return
	}

	var backupFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, backupBase+".backup.") {
			backupFiles = append(backupFiles, filepath.Join(backupDir, name))
		}
	}

	// If we have more than 3 backups, remove the oldest ones
	if len(backupFiles) > 3 {
		type fileInfo struct {
			path string
			time time.Time
		}
		var filesWithTime []fileInfo
		for _, path := range backupFiles {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			filesWithTime = append(filesWithTime, fileInfo{
				path: path,
				time: info.ModTime(),
			})
		}

		// Sort by time (oldest first)
		for i := 0; i < len(filesWithTime)-1; i++ {
			for j := i + 1; j < len(filesWithTime); j++ {
				if filesWithTime[i].time.After(filesWithTime[j].time) {
					filesWithTime[i], filesWithTime[j] = filesWithTime[j], filesWithTime[i]
				}
			}
		}

		// Remove oldest files (keep last 3)
		toRemove := len(filesWithTime) - 3
		for i := 0; i < toRemove; i++ {
			if err := os.Remove(filesWithTime[i].path); err != nil {
				logger.WithError(err).WithField("path", filesWithTime[i].path).Debug("Failed to remove old backup")
			} else {
				logger.WithField("path", filesWithTime[i].path).Debug("Removed old backup file")
			}
		}
		logger.WithField("removed", toRemove).WithField("kept", 3).Info("Cleaned up old backup files")
	}
}

// checkRecentUpdate checks if we updated recently to prevent update loops
func checkRecentUpdate() error {
	updateMarkerPath := filepath.Join(config.DefaultConfigDir, ".last_update_timestamp")

	// Check if marker file exists
	info, err := os.Stat(updateMarkerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return nil
	}

	// Check if update was within last 5 minutes
	timeSinceUpdate := time.Since(info.ModTime())
	if timeSinceUpdate < 5*time.Minute {
		return fmt.Errorf("update was performed %v ago, waiting to prevent update loop", timeSinceUpdate)
	}

	return nil
}

// markRecentUpdate creates a timestamp file to mark that we just updated
func markRecentUpdate() {
	updateMarkerPath := filepath.Join(config.DefaultConfigDir, ".last_update_timestamp")

	// Ensure directory exists
	if err := os.MkdirAll(config.DefaultConfigDir, 0755); err != nil {
		logger.WithError(err).Debug("Could not create PatchMon config directory (non-critical)")
		return
	}

	// Create or update the timestamp file
	file, err := os.Create(updateMarkerPath)
	if err != nil {
		logger.WithError(err).Debug("Could not create update marker file (non-critical)")
		return
	}
	if err := file.Close(); err != nil {
		logger.WithError(err).Debug("Could not close update marker file (non-critical)")
	}

	logger.Debug("Marked recent update to prevent update loops")
}
