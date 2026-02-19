package system

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows/registry"

	"patchmon-agent/internal/constants"
	"patchmon-agent/pkg/models"
)

// Registry key path for Windows version information
const ntCurrentVersionKey = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`

// Detector handles system information detection
type Detector struct {
	logger *logrus.Logger
}

// New creates a new system detector
func New(logger *logrus.Logger) *Detector {
	return &Detector{
		logger: logger,
	}
}

// DetectOS detects the operating system type and version from the Windows registry.
//
// Returns:
//   - osType: base product name, e.g. "Windows 10", "Windows 11", "Windows Server 2022"
//   - osVersion: feature update version, e.g. "23H2", "24H2", or build number as fallback
func (d *Detector) DetectOS() (osType, osVersion string, err error) {
	productName, displayVersion, currentBuild, err := readNTVersionFromRegistry()
	if err != nil {
		d.logger.WithError(err).Warn("Failed to read OS info from registry, falling back to gopsutil")
		return d.detectOSFallback()
	}

	// Extract base product name (e.g. "Windows 10", "Windows 11", "Windows Server 2022")
	osType = extractBaseProductName(productName)
	if osType == "" {
		osType = productName // use full product name if extraction fails
	}

	// Use DisplayVersion (e.g. "23H2") if available, otherwise fall back to CurrentBuild
	osVersion = displayVersion
	if osVersion == "" {
		osVersion = currentBuild
	}
	if osVersion == "" {
		osVersion = "Unknown"
	}

	d.logger.WithFields(logrus.Fields{
		"productName":    productName,
		"displayVersion": displayVersion,
		"currentBuild":   currentBuild,
		"osType":         osType,
		"osVersion":      osVersion,
	}).Debug("Detected OS information from registry")

	return osType, osVersion, nil
}

// readNTVersionFromRegistry reads Windows version info from the registry.
// Returns productName, displayVersion, currentBuild, and any error.
func readNTVersionFromRegistry() (productName, displayVersion, currentBuild string, err error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, ntCurrentVersionKey, registry.QUERY_VALUE)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to open registry key %s: %w", ntCurrentVersionKey, err)
	}
	defer k.Close()

	productName, _, _ = k.GetStringValue("ProductName")
	displayVersion, _, _ = k.GetStringValue("DisplayVersion")
	currentBuild, _, _ = k.GetStringValue("CurrentBuild")

	if productName == "" {
		return "", "", "", fmt.Errorf("ProductName not found in registry")
	}

	return productName, displayVersion, currentBuild, nil
}

// extractBaseProductName extracts the base OS name from a full ProductName string.
// Examples:
//
//	"Windows 10 Pro"                    → "Windows 10"
//	"Windows 11 Enterprise"             → "Windows 11"
//	"Windows Server 2022 Standard"      → "Windows Server 2022"
//	"Windows Server 2019 Datacenter"    → "Windows Server 2019"
func extractBaseProductName(productName string) string {
	// Match "Windows Server YYYY" or "Windows NN"
	re := regexp.MustCompile(`^(Windows\s+Server\s+\d{4}|Windows\s+\d+)`)
	match := re.FindString(productName)
	return match
}

// detectOSFallback uses gopsutil as a fallback for OS detection
func (d *Detector) detectOSFallback() (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get host info via gopsutil")
		return "Windows", "Unknown", err
	}

	osType := info.Platform
	if osType == "" {
		osType = "Windows"
	}

	osVersion := info.PlatformVersion
	if osVersion == "" {
		osVersion = "Unknown"
	}

	return osType, osVersion, nil
}

// GetKernelVersion returns the full Windows build string: "10.0.{CurrentBuild}.{UBR}"
// e.g. "10.0.19045.3803"
func (d *Detector) GetKernelVersion() string {
	version, err := readKernelVersionFromRegistry()
	if err != nil {
		d.logger.WithError(err).Warn("Failed to read kernel version from registry, falling back to gopsutil")
		return d.getKernelVersionFallback()
	}
	return version
}

// readKernelVersionFromRegistry reads the full build string from the registry.
func readKernelVersionFromRegistry() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, ntCurrentVersionKey, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	currentBuild, _, err := k.GetStringValue("CurrentBuild")
	if err != nil {
		return "", fmt.Errorf("failed to read CurrentBuild: %w", err)
	}

	ubr, _, err := k.GetIntegerValue("UBR")
	if err != nil {
		// UBR may not exist on older systems; return without it
		return fmt.Sprintf("10.0.%s", currentBuild), nil
	}

	return fmt.Sprintf("10.0.%s.%d", currentBuild, ubr), nil
}

// getKernelVersionFallback uses gopsutil to get the kernel version
func (d *Detector) getKernelVersionFallback() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get kernel version via gopsutil")
		return constants.ErrUnknownValue
	}

	return info.KernelVersion
}

// GetLatestInstalledKernel returns the Windows build version.
// On Windows, there is no separate kernel package — the kernel version is the
// same as the OS build version, so this returns the same value as GetKernelVersion().
func (d *Detector) GetLatestInstalledKernel() string {
	return d.GetKernelVersion()
}

// GetSystemInfo assembles all system information into a SystemInfo struct.
func (d *Detector) GetSystemInfo() models.SystemInfo {
	d.logger.Debug("Beginning system information collection")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info := models.SystemInfo{
		KernelVersion: d.GetKernelVersion(),
		SELinuxStatus: getSELinuxStatus(),
		SystemUptime:  d.getSystemUptime(ctx),
		LoadAverage:   getLoadAverage(),
	}

	d.logger.WithFields(logrus.Fields{
		"kernel": info.KernelVersion,
		"uptime": info.SystemUptime,
	}).Debug("Collected system information")

	return info
}

// GetArchitecture returns the system architecture (e.g. "amd64", "arm64")
func (d *Detector) GetArchitecture() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get architecture")
		return constants.ArchUnknown
	}

	return info.KernelArch
}

// GetHostname returns the system hostname
func (d *Detector) GetHostname() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := host.InfoWithContext(ctx)
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get hostname via gopsutil")
		// Fallback to os.Hostname
		return os.Hostname()
	}

	return info.Hostname, nil
}

// GetIPAddress gets the primary non-loopback IPv4 address
func (d *Detector) GetIPAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get network interfaces")
		return ""
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
					return ipnet.IP.String()
				}
			}
		}
	}

	return ""
}

// GetMachineID returns the system's machine ID (MachineGuid from registry via gopsutil)
func (d *Detector) GetMachineID() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// On Windows, gopsutil reads the MachineGuid from the registry
	hostID, err := host.HostIDWithContext(ctx)
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get host ID, using hostname as fallback")
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
		return "unknown"
	}

	return hostID
}

// getSELinuxStatus returns the SELinux status.
// SELinux does not exist on Windows, so this always returns "disabled".
func getSELinuxStatus() string {
	return "disabled"
}

// getLoadAverage returns the system load average.
// Load average is a Unix/Linux concept and does not exist on Windows.
// We return [0.0, 0.0, 0.0] as a placeholder to satisfy the API contract.
func getLoadAverage() []float64 {
	return []float64{0.0, 0.0, 0.0}
}

// getSystemUptime gets the system uptime as a human-readable string
func (d *Detector) getSystemUptime(ctx context.Context) string {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		d.logger.WithError(err).Warn("Failed to get uptime")
		return "Unknown"
	}

	return FormatUptime(info.Uptime)
}

// FormatUptime converts an uptime in seconds to a human-readable string.
// Exported for testing.
func FormatUptime(uptimeSeconds uint64) string {
	uptime := time.Duration(uptimeSeconds) * time.Second

	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
	}
	return fmt.Sprintf("%d minutes", minutes)
}
