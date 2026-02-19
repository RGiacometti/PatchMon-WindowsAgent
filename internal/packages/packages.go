package packages

import (
	"patchmon-agent/pkg/models"

	"github.com/sirupsen/logrus"
)

// Manager handles package information collection
type Manager struct {
	logger         *logrus.Logger
	windowsManager *WindowsUpdateManager
}

// New creates a new package manager
func New(logger *logrus.Logger) *Manager {
	return &Manager{
		logger:         logger,
		windowsManager: NewWindowsUpdateManager(logger),
	}
}

// GetPackages gets package information from Windows Update.
// It collects both installed updates and available (pending) updates.
func (m *Manager) GetPackages() ([]models.Package, error) {
	// Get installed updates
	installed, err := m.windowsManager.GetInstalledUpdates()
	if err != nil {
		m.logger.Warnf("Failed to get installed updates: %v", err)
		installed = []models.Package{}
	}

	// Get available updates
	available, err := m.windowsManager.GetAvailableUpdates()
	if err != nil {
		m.logger.Warnf("Failed to get available updates: %v", err)
		available = []models.Package{}
	}

	// Combine: installed updates (NeedsUpdate=false) + available updates (NeedsUpdate=true)
	allPackages := make([]models.Package, 0, len(installed)+len(available))
	allPackages = append(allPackages, installed...)
	allPackages = append(allPackages, available...)

	m.logger.Infof("Found %d installed updates and %d available updates", len(installed), len(available))

	return allPackages, nil
}

// CombinePackageData combines and deduplicates installed and upgradable package lists
func CombinePackageData(installedPackages map[string]models.Package, upgradablePackages []models.Package) []models.Package {
	packages := make([]models.Package, 0)
	upgradableMap := make(map[string]bool)

	// First, add all upgradable packages
	for _, pkg := range upgradablePackages {
		// Preserve description from installed packages if available and not present in upgradable
		if installedPkg, exists := installedPackages[pkg.Name]; exists {
			if pkg.Description == "" {
				pkg.Description = installedPkg.Description
			}
		}
		packages = append(packages, pkg)
		upgradableMap[pkg.Name] = true
	}

	// Then add installed packages that are not upgradable
	for packageName, pkg := range installedPackages {
		if !upgradableMap[packageName] {
			packages = append(packages, models.Package{
				Name:             pkg.Name,
				CurrentVersion:   pkg.CurrentVersion,
				NeedsUpdate:      false,
				IsSecurityUpdate: false,
			})
		}
	}

	return packages
}
