package system

import (
	"os"
	"os/exec"
	"strings"
)

// CheckRebootRequired checks if the system requires a reboot
// Returns (needsReboot bool, reason string)
func (d *Detector) CheckRebootRequired() (bool, string) {
	// Check Debian/Ubuntu - reboot-required flag file
	if _, err := os.Stat("/var/run/reboot-required"); err == nil {
		d.logger.Debug("Reboot required: /var/run/reboot-required file exists")
		return true, "Reboot flag file exists"
	}

	// Check RHEL/Fedora - needs-restarting utility
	if needsRestart, reason := d.checkNeedsRestarting(); needsRestart {
		d.logger.WithField("reason", reason).Debug("Reboot required: needs-restarting check")
		return true, reason
	}

	// Universal kernel check - compare running vs latest installed
	runningKernel := d.getRunningKernel()
	latestKernel := d.getLatestInstalledKernel()

	if runningKernel != latestKernel && latestKernel != "" {
		d.logger.WithFields(map[string]interface{}{
			"running": runningKernel,
			"latest":  latestKernel,
		}).Debug("Reboot required: kernel version mismatch")
		return true, "Kernel version mismatch"
	}

	d.logger.Debug("No reboot required")
	return false, "No reboot required"
}

// checkNeedsRestarting checks using needs-restarting command (RHEL/Fedora)
func (d *Detector) checkNeedsRestarting() (bool, string) {
	// Check if needs-restarting command exists
	if _, err := exec.LookPath("needs-restarting"); err != nil {
		d.logger.Debug("needs-restarting command not found, skipping check")
		return false, ""
	}

	cmd := exec.Command("needs-restarting", "-r")
	if err := cmd.Run(); err != nil {
		// Exit code != 0 means reboot is needed
		if _, ok := err.(*exec.ExitError); ok {
			return true, "needs-restarting indicates reboot needed"
		}
		d.logger.WithError(err).Debug("needs-restarting command failed")
	}

	return false, ""
}

// getRunningKernel gets the currently running kernel version
func (d *Detector) getRunningKernel() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get running kernel version")
		return ""
	}
	return strings.TrimSpace(string(output))
}

// GetLatestInstalledKernel gets the latest installed kernel version (public method)
func (d *Detector) GetLatestInstalledKernel() string {
	return d.getLatestInstalledKernel()
}

// getLatestInstalledKernel gets the latest installed kernel version
func (d *Detector) getLatestInstalledKernel() string {
	// Try different methods based on common distro patterns

	// Method 1: Debian/Ubuntu - check /boot for vmlinuz files
	if latest := d.getLatestKernelFromBoot(); latest != "" {
		return latest
	}

	// Method 2: RHEL/Fedora - use rpm to query installed kernels
	if latest := d.getLatestKernelFromRPM(); latest != "" {
		return latest
	}

	// Method 3: Try dpkg for Debian-based systems
	if latest := d.getLatestKernelFromDpkg(); latest != "" {
		return latest
	}

	d.logger.Debug("Could not determine latest installed kernel")
	return ""
}

// getLatestKernelFromBoot scans /boot for vmlinuz files
func (d *Detector) getLatestKernelFromBoot() string {
	entries, err := os.ReadDir("/boot")
	if err != nil {
		d.logger.WithError(err).Debug("Failed to read /boot directory")
		return ""
	}

	var latestVersion string
	for _, entry := range entries {
		name := entry.Name()
		// Look for vmlinuz-* files
		if strings.HasPrefix(name, "vmlinuz-") {
			version := strings.TrimPrefix(name, "vmlinuz-")
			// Skip generic/recovery kernels if we already have a version
			if latestVersion != "" && (strings.Contains(version, "generic") || strings.Contains(version, "recovery")) {
				continue
			}
			latestVersion = version
		}
	}

	return latestVersion
}

// getLatestKernelFromRPM queries RPM for installed kernel packages
func (d *Detector) getLatestKernelFromRPM() string {
	// Check if rpm command exists
	if _, err := exec.LookPath("rpm"); err != nil {
		return ""
	}

	cmd := exec.Command("rpm", "-q", "kernel", "--last")
	output, err := cmd.Output()
	if err != nil {
		d.logger.WithError(err).Debug("Failed to query RPM for kernel packages")
		return ""
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 && lines[0] != "" {
		// Parse first line which should be the latest kernel
		// Format: kernel-VERSION DATE
		parts := strings.Fields(lines[0])
		if len(parts) > 0 {
			// Extract version from kernel-X.Y.Z
			kernelPkg := parts[0]
			version := strings.TrimPrefix(kernelPkg, "kernel-")
			return version
		}
	}

	return ""
}

// getLatestKernelFromDpkg queries dpkg for installed kernel packages
func (d *Detector) getLatestKernelFromDpkg() string {
	// Check if dpkg command exists
	if _, err := exec.LookPath("dpkg"); err != nil {
		return ""
	}

	cmd := exec.Command("dpkg", "-l")
	output, err := cmd.Output()
	if err != nil {
		d.logger.WithError(err).Debug("Failed to query dpkg for kernel packages")
		return ""
	}

	var latestVersion string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Look for installed kernel image packages
		if fields[0] == "ii" && strings.HasPrefix(fields[1], "linux-image-") {
			// Extract version from package name
			// Format: linux-image-VERSION or linux-image-X.Y.Z-N-generic
			pkgName := fields[1]
			version := strings.TrimPrefix(pkgName, "linux-image-")

			// Skip meta packages
			if version == "generic" || version == "lowlatency" {
				continue
			}

			latestVersion = version
		}
	}

	return latestVersion
}
