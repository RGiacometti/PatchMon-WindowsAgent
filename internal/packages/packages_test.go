package packages

import (
	"testing"

	"patchmon-agent/pkg/models"

	"github.com/sirupsen/logrus"
)

func TestNew(t *testing.T) {
	logger := logrus.New()
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

// TestGetPackages_Integration is an integration test that verifies GetPackages
// returns a valid (non-nil) slice. Requires a Windows machine with WUA service running.
func TestGetPackages_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	mgr := New(logger)

	packages, err := mgr.GetPackages()
	if err != nil {
		t.Fatalf("GetPackages returned error: %v", err)
	}
	if packages == nil {
		t.Fatal("GetPackages returned nil slice, expected non-nil")
	}

	t.Logf("GetPackages returned %d total packages", len(packages))

	// Count installed vs available
	installedCount := 0
	availableCount := 0
	securityCount := 0
	for _, pkg := range packages {
		if pkg.NeedsUpdate {
			availableCount++
		} else {
			installedCount++
		}
		if pkg.IsSecurityUpdate {
			securityCount++
		}
	}
	t.Logf("Installed: %d, Available: %d, Security: %d", installedCount, availableCount, securityCount)
}

func TestCombinePackageData(t *testing.T) {
	tests := []struct {
		name               string
		installed          map[string]models.Package
		upgradable         []models.Package
		expectedCount      int
		expectedNeedsCount int
	}{
		{
			name:               "empty inputs",
			installed:          map[string]models.Package{},
			upgradable:         []models.Package{},
			expectedCount:      0,
			expectedNeedsCount: 0,
		},
		{
			name: "only installed packages",
			installed: map[string]models.Package{
				"KB5034441": {Name: "KB5034441", CurrentVersion: "1.0", Description: "Security Update"},
				"KB5034442": {Name: "KB5034442", CurrentVersion: "2.0", Description: "Feature Update"},
			},
			upgradable:         []models.Package{},
			expectedCount:      2,
			expectedNeedsCount: 0,
		},
		{
			name:      "only upgradable packages",
			installed: map[string]models.Package{},
			upgradable: []models.Package{
				{Name: "KB5034443", AvailableVersion: "3.0", NeedsUpdate: true},
			},
			expectedCount:      1,
			expectedNeedsCount: 1,
		},
		{
			name: "mixed with overlap",
			installed: map[string]models.Package{
				"KB5034441": {Name: "KB5034441", CurrentVersion: "1.0", Description: "Security Update"},
				"KB5034442": {Name: "KB5034442", CurrentVersion: "2.0", Description: "Feature Update"},
			},
			upgradable: []models.Package{
				{Name: "KB5034441", AvailableVersion: "1.1", NeedsUpdate: true},
			},
			expectedCount:      2, // 1 upgradable + 1 installed-only
			expectedNeedsCount: 1,
		},
		{
			name: "description preserved from installed when missing in upgradable",
			installed: map[string]models.Package{
				"KB5034441": {Name: "KB5034441", CurrentVersion: "1.0", Description: "Security Update for Windows"},
			},
			upgradable: []models.Package{
				{Name: "KB5034441", AvailableVersion: "1.1", NeedsUpdate: true, Description: ""},
			},
			expectedCount:      1,
			expectedNeedsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CombinePackageData(tt.installed, tt.upgradable)

			if len(result) != tt.expectedCount {
				t.Errorf("expected %d packages, got %d", tt.expectedCount, len(result))
			}

			needsCount := 0
			for _, pkg := range result {
				if pkg.NeedsUpdate {
					needsCount++
				}
			}
			if needsCount != tt.expectedNeedsCount {
				t.Errorf("expected %d packages needing update, got %d", tt.expectedNeedsCount, needsCount)
			}
		})
	}
}

func TestCombinePackageData_DescriptionPreserved(t *testing.T) {
	installed := map[string]models.Package{
		"KB5034441": {Name: "KB5034441", CurrentVersion: "1.0", Description: "Security Update for Windows"},
	}
	upgradable := []models.Package{
		{Name: "KB5034441", AvailableVersion: "1.1", NeedsUpdate: true, Description: ""},
	}

	result := CombinePackageData(installed, upgradable)

	if len(result) != 1 {
		t.Fatalf("expected 1 package, got %d", len(result))
	}
	if result[0].Description != "Security Update for Windows" {
		t.Errorf("expected description to be preserved from installed package, got %q", result[0].Description)
	}
}
