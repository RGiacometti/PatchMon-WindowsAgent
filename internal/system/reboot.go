package system

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Registry paths for pending reboot indicators
const (
	rebootRequiredKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`
	rebootPendingKey  = `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`
	sessionManagerKey = `SYSTEM\CurrentControlSet\Control\Session Manager`
)

// CheckRebootRequired checks if the system requires a reboot by inspecting
// Windows registry keys for pending reboot indicators.
//
// Returns:
//   - needsReboot: true if any reboot indicator is found
//   - reason: semicolon-separated description of all detected reasons
func (d *Detector) CheckRebootRequired() (bool, string) {
	reasons := []string{}

	// 1. Check Windows Update pending reboot
	if registryKeyExists(rebootRequiredKey) {
		reasons = append(reasons, "Windows Update pending reboot")
	}

	// 2. Check Component Based Servicing pending reboot
	if registryKeyExists(rebootPendingKey) {
		reasons = append(reasons, "Component servicing pending reboot")
	}

	// 3. Check Pending File Rename Operations
	if registryValueExists(sessionManagerKey, "PendingFileRenameOperations") {
		reasons = append(reasons, "Pending file rename operations")
	}

	if len(reasons) > 0 {
		reason := strings.Join(reasons, "; ")
		d.logger.WithField("reason", reason).Debug("Reboot required")
		return true, reason
	}

	d.logger.Debug("No reboot required")
	return false, ""
}

// registryKeyExists checks if a registry key exists under HKLM.
func registryKeyExists(keyPath string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

// registryValueExists checks if a specific value exists under a registry key in HKLM.
func registryValueExists(keyPath, valueName string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(valueName)
	if err == nil {
		return true
	}

	// PendingFileRenameOperations can be REG_MULTI_SZ, try GetStringsValue
	_, _, err = k.GetStringsValue(valueName)
	return err == nil
}

// BuildRebootReason is a helper that builds a reboot reason string from a list
// of individual reasons. Exported for testing.
func BuildRebootReason(reasons []string) string {
	return strings.Join(reasons, "; ")
}
