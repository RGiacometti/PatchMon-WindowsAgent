package system

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestBuildRebootReason(t *testing.T) {
	tests := []struct {
		name    string
		reasons []string
		want    string
	}{
		{
			name:    "no reasons",
			reasons: []string{},
			want:    "",
		},
		{
			name:    "single reason",
			reasons: []string{"Windows Update pending reboot"},
			want:    "Windows Update pending reboot",
		},
		{
			name:    "two reasons",
			reasons: []string{"Windows Update pending reboot", "Component servicing pending reboot"},
			want:    "Windows Update pending reboot; Component servicing pending reboot",
		},
		{
			name: "all three reasons",
			reasons: []string{
				"Windows Update pending reboot",
				"Component servicing pending reboot",
				"Pending file rename operations",
			},
			want: "Windows Update pending reboot; Component servicing pending reboot; Pending file rename operations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRebootReason(tt.reasons)
			if got != tt.want {
				t.Errorf("BuildRebootReason(%v) = %q, want %q", tt.reasons, got, tt.want)
			}
		})
	}
}

func TestBuildRebootReason_SemicolonSeparator(t *testing.T) {
	reasons := []string{"reason1", "reason2", "reason3"}
	result := BuildRebootReason(reasons)

	parts := strings.Split(result, "; ")
	if len(parts) != 3 {
		t.Errorf("Expected 3 parts separated by '; ', got %d: %q", len(parts), result)
	}
}

// TestRegistryKeyExists_KnownKey tests that registryKeyExists returns true for
// a well-known registry key that always exists on Windows.
func TestRegistryKeyExists_KnownKey(t *testing.T) {
	// This key always exists on Windows
	exists := registryKeyExists(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`)
	if !exists {
		t.Error("registryKeyExists() returned false for a known existing key")
	}
}

// TestRegistryKeyExists_NonExistentKey tests that registryKeyExists returns false
// for a key that does not exist.
func TestRegistryKeyExists_NonExistentKey(t *testing.T) {
	exists := registryKeyExists(`SOFTWARE\NonExistent\Key\That\Should\Not\Exist\PatchMonTest`)
	if exists {
		t.Error("registryKeyExists() returned true for a non-existent key")
	}
}

// TestRegistryValueExists_KnownValue tests that registryValueExists returns true
// for a well-known registry value.
func TestRegistryValueExists_KnownValue(t *testing.T) {
	// ProductName always exists under this key
	exists := registryValueExists(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	if !exists {
		t.Error("registryValueExists() returned false for ProductName which should exist")
	}
}

// TestRegistryValueExists_NonExistentValue tests that registryValueExists returns
// false for a value that does not exist.
func TestRegistryValueExists_NonExistentValue(t *testing.T) {
	exists := registryValueExists(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "PatchMonNonExistentValue12345")
	if exists {
		t.Error("registryValueExists() returned true for a non-existent value")
	}
}

// TestCheckRebootRequired_Integration is an integration test that runs the full
// reboot check on the current system. We can't predict the result, but we verify
// it doesn't panic and returns valid types.
func TestCheckRebootRequired_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	d := New(logger)
	needsReboot, reason := d.CheckRebootRequired()

	if needsReboot {
		if reason == "" {
			t.Error("CheckRebootRequired() returned needsReboot=true but empty reason")
		}
		t.Logf("System needs reboot: %s", reason)
	} else {
		if reason != "" {
			t.Errorf("CheckRebootRequired() returned needsReboot=false but non-empty reason: %q", reason)
		}
		t.Log("System does not need reboot")
	}
}
