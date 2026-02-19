package packages

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	return logger
}

func TestNewWindowsUpdateManager(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateManager(logger)

	if mgr == nil {
		t.Fatal("NewWindowsUpdateManager returned nil")
	}
	if mgr.logger != logger {
		t.Error("WindowsUpdateManager logger not set correctly")
	}
}

// TestParseUpdateInstalledCriteria verifies that parseUpdate correctly sets
// NeedsUpdate=false and populates CurrentVersion for installed update criteria.
// Note: This test requires a real IDispatch COM object, so it is an integration test.
// We test the criteria-based logic indirectly through the full integration tests below.

// TestGetInstalledUpdates_Integration is an integration test that calls the real
// Windows Update Agent COM API. It requires a Windows machine with the WUA service running.
func TestGetInstalledUpdates_Integration(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateManager(logger)

	installed, err := mgr.GetInstalledUpdates()
	if err != nil {
		t.Fatalf("GetInstalledUpdates failed: %v", err)
	}

	// On any Windows machine, there should be at least some installed updates
	t.Logf("Found %d installed updates", len(installed))

	// Verify all installed updates have NeedsUpdate=false
	for _, pkg := range installed {
		if pkg.NeedsUpdate {
			t.Errorf("Installed update %q has NeedsUpdate=true, expected false", pkg.Name)
		}
		if pkg.CurrentVersion == "" {
			t.Errorf("Installed update %q has empty CurrentVersion", pkg.Name)
		}
		if pkg.Name == "" {
			t.Error("Found installed update with empty Name")
		}
	}
}

// TestGetAvailableUpdates_Integration is an integration test that calls the real
// Windows Update Agent COM API. It requires a Windows machine with the WUA service running.
// Note: This search can take 30-60 seconds on some machines.
func TestGetAvailableUpdates_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping available updates test in short mode (can take 30-60 seconds)")
	}

	logger := newTestLogger()
	mgr := NewWindowsUpdateManager(logger)

	available, err := mgr.GetAvailableUpdates()
	if err != nil {
		t.Fatalf("GetAvailableUpdates failed: %v", err)
	}

	t.Logf("Found %d available updates", len(available))

	// Verify all available updates have NeedsUpdate=true
	for _, pkg := range available {
		if !pkg.NeedsUpdate {
			t.Errorf("Available update %q has NeedsUpdate=false, expected true", pkg.Name)
		}
		if pkg.AvailableVersion == "" {
			t.Errorf("Available update %q has empty AvailableVersion", pkg.Name)
		}
		if pkg.Name == "" {
			t.Error("Found available update with empty Name")
		}
	}
}

// TestSearchUpdates_InvalidCriteria verifies that an invalid search criteria
// returns an error rather than panicking.
func TestSearchUpdates_InvalidCriteria(t *testing.T) {
	logger := newTestLogger()
	mgr := NewWindowsUpdateManager(logger)

	_, err := mgr.searchUpdates("InvalidCriteria=BOGUS")
	if err == nil {
		t.Error("Expected error for invalid search criteria, got nil")
	} else {
		t.Logf("Got expected error for invalid criteria: %v", err)
	}
}
