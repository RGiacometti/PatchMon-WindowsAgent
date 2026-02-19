package repositories

import (
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/registry"

	"patchmon-agent/internal/constants"
	"patchmon-agent/pkg/models"
)

// WindowsUpdateSourceManager detects Windows Update configuration sources
type WindowsUpdateSourceManager struct {
	logger *logrus.Logger
}

// NewWindowsUpdateSourceManager creates a new WindowsUpdateSourceManager
func NewWindowsUpdateSourceManager(logger *logrus.Logger) *WindowsUpdateSourceManager {
	return &WindowsUpdateSourceManager{logger: logger}
}

// GetSources returns the configured Windows Update sources (WSUS, Microsoft Update, etc.)
func (w *WindowsUpdateSourceManager) GetSources() ([]models.Repository, error) {
	repos := []models.Repository{}

	// Check for WSUS server
	wsusServer := w.getWSUSServer()
	if wsusServer != "" {
		w.logger.Debugf("WSUS server detected: %s", wsusServer)
		repos = append(repos, models.Repository{
			Name:      "WSUS",
			URL:       wsusServer,
			RepoType:  constants.RepoTypeWindowsUpdate,
			IsEnabled: true,
			IsSecure:  strings.HasPrefix(wsusServer, "https://"),
		})
	}

	// Check if Microsoft Update is enabled (vs just Windows Update)
	if w.isMicrosoftUpdateEnabled() {
		w.logger.Debug("Microsoft Update service is enabled")
		repos = append(repos, models.Repository{
			Name:      "Microsoft Update",
			URL:       "https://update.microsoft.com",
			RepoType:  constants.RepoTypeWindowsUpdate,
			IsEnabled: true,
			IsSecure:  true,
		})
	} else {
		w.logger.Debug("Using standard Windows Update")
		repos = append(repos, models.Repository{
			Name:      "Windows Update",
			URL:       "https://windowsupdate.microsoft.com",
			RepoType:  constants.RepoTypeWindowsUpdate,
			IsEnabled: true,
			IsSecure:  true,
		})
	}

	return repos, nil
}

// getWSUSServer reads the WSUS server URL from the Windows registry.
// Returns empty string if no WSUS server is configured.
func (w *WindowsUpdateSourceManager) getWSUSServer() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`,
		registry.QUERY_VALUE)
	if err != nil {
		w.logger.Debug("No WSUS registry key found (not configured via Group Policy)")
		return ""
	}
	defer key.Close()

	server, _, err := key.GetStringValue("WUServer")
	if err != nil {
		w.logger.Debug("WSUS registry key exists but WUServer value not found")
		return ""
	}
	return server
}

// isMicrosoftUpdateEnabled checks whether the Microsoft Update service is registered.
// Microsoft Update provides updates for all Microsoft products (Office, SQL Server, etc.),
// while plain Windows Update only covers the OS.
func (w *WindowsUpdateSourceManager) isMicrosoftUpdateEnabled() bool {
	// The Microsoft Update service is identified by GUID 7971f918-a847-4430-9279-4a52d1efe18d
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Services\7971f918-a847-4430-9279-4a52d1efe18d`,
		registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	key.Close()
	return true
}
