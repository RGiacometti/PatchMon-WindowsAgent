package repositories

import (
	"testing"

	"patchmon-agent/internal/constants"

	"github.com/sirupsen/logrus"
)

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}

func TestNewWindowsUpdateSourceManager(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateSourceManager(logger)

	if mgr == nil {
		t.Fatal("NewWindowsUpdateSourceManager returned nil")
	}
	if mgr.logger != logger {
		t.Error("WindowsUpdateSourceManager logger not set correctly")
	}
}

func TestNew(t *testing.T) {
	logger := newTestLogger()
	mgr := New(logger)

	if mgr == nil {
		t.Fatal("New returned nil")
	}
	if mgr.logger != logger {
		t.Error("Manager logger not set correctly")
	}
	if mgr.windowsManager == nil {
		t.Error("Manager windowsManager not initialized")
	}
}

// TestGetWSUSServer tests the WSUS server detection.
// On most dev machines without Group Policy, this will return empty string.
func TestGetWSUSServer(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateSourceManager(logger)

	server := mgr.getWSUSServer()
	// We can't assert a specific value since it depends on the machine's configuration.
	// Just verify it doesn't panic and returns a string.
	t.Logf("WSUS server: %q", server)
}

// TestIsMicrosoftUpdateEnabled tests the Microsoft Update service detection.
func TestIsMicrosoftUpdateEnabled(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateSourceManager(logger)

	enabled := mgr.isMicrosoftUpdateEnabled()
	t.Logf("Microsoft Update enabled: %v", enabled)
}

// TestGetSources_AlwaysReturnsAtLeastOne verifies that GetSources always returns
// at least one repository (either Windows Update or Microsoft Update).
func TestGetSources_AlwaysReturnsAtLeastOne(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateSourceManager(logger)

	repos, err := mgr.GetSources()
	if err != nil {
		t.Fatalf("GetSources returned error: %v", err)
	}

	if len(repos) == 0 {
		t.Fatal("GetSources returned empty slice, expected at least one repository")
	}

	// Verify all repos have the correct type
	for _, repo := range repos {
		if repo.RepoType != constants.RepoTypeWindowsUpdate {
			t.Errorf("Repository %q has type %q, expected %q", repo.Name, repo.RepoType, constants.RepoTypeWindowsUpdate)
		}
		if repo.Name == "" {
			t.Error("Found repository with empty Name")
		}
		if repo.URL == "" {
			t.Error("Found repository with empty URL")
		}
		if !repo.IsEnabled {
			t.Errorf("Repository %q is not enabled", repo.Name)
		}
	}

	// At least one should be Windows Update or Microsoft Update
	hasUpdateSource := false
	for _, repo := range repos {
		if repo.Name == "Windows Update" || repo.Name == "Microsoft Update" {
			hasUpdateSource = true
			break
		}
	}
	if !hasUpdateSource {
		t.Error("Expected at least one 'Windows Update' or 'Microsoft Update' repository")
	}

	t.Logf("Found %d repositories:", len(repos))
	for _, repo := range repos {
		t.Logf("  - %s (%s) secure=%v", repo.Name, repo.URL, repo.IsSecure)
	}
}

// TestGetRepositories_Integration verifies the full Manager.GetRepositories flow.
func TestGetRepositories_Integration(t *testing.T) {
	logger := newTestLogger()
	mgr := New(logger)

	repos, err := mgr.GetRepositories()
	if err != nil {
		t.Fatalf("GetRepositories returned error: %v", err)
	}

	if repos == nil {
		t.Fatal("GetRepositories returned nil slice, expected non-nil")
	}

	if len(repos) == 0 {
		t.Fatal("GetRepositories returned empty slice, expected at least one repository")
	}

	t.Logf("GetRepositories returned %d repositories", len(repos))
}
