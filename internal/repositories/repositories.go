package repositories

import (
	"patchmon-agent/pkg/models"

	"github.com/sirupsen/logrus"
)

// Manager handles repository information collection
type Manager struct {
	logger         *logrus.Logger
	windowsManager *WindowsUpdateSourceManager
}

// New creates a new repository manager
func New(logger *logrus.Logger) *Manager {
	return &Manager{
		logger:         logger,
		windowsManager: NewWindowsUpdateSourceManager(logger),
	}
}

// GetRepositories gets repository information from Windows Update sources
func (m *Manager) GetRepositories() ([]models.Repository, error) {
	repos, err := m.windowsManager.GetSources()
	if err != nil {
		m.logger.Warnf("Failed to get Windows Update sources: %v", err)
		return []models.Repository{}, nil
	}
	return repos, nil
}
