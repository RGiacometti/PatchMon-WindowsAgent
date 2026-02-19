package system

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestExtractBaseProductName(t *testing.T) {
	tests := []struct {
		name        string
		productName string
		want        string
	}{
		{
			name:        "Windows 10 Pro",
			productName: "Windows 10 Pro",
			want:        "Windows 10",
		},
		{
			name:        "Windows 10 Enterprise",
			productName: "Windows 10 Enterprise",
			want:        "Windows 10",
		},
		{
			name:        "Windows 11 Home",
			productName: "Windows 11 Home",
			want:        "Windows 11",
		},
		{
			name:        "Windows 11 Enterprise",
			productName: "Windows 11 Enterprise",
			want:        "Windows 11",
		},
		{
			name:        "Windows 11 Pro for Workstations",
			productName: "Windows 11 Pro for Workstations",
			want:        "Windows 11",
		},
		{
			name:        "Windows Server 2019 Standard",
			productName: "Windows Server 2019 Standard",
			want:        "Windows Server 2019",
		},
		{
			name:        "Windows Server 2019 Datacenter",
			productName: "Windows Server 2019 Datacenter",
			want:        "Windows Server 2019",
		},
		{
			name:        "Windows Server 2022 Standard",
			productName: "Windows Server 2022 Standard",
			want:        "Windows Server 2022",
		},
		{
			name:        "Windows Server 2022 Datacenter",
			productName: "Windows Server 2022 Datacenter",
			want:        "Windows Server 2022",
		},
		{
			name:        "Windows Server 2025 Standard",
			productName: "Windows Server 2025 Standard",
			want:        "Windows Server 2025",
		},
		{
			name:        "bare Windows 10",
			productName: "Windows 10",
			want:        "Windows 10",
		},
		{
			name:        "empty string",
			productName: "",
			want:        "",
		},
		{
			name:        "unrecognized format",
			productName: "SomeOtherOS",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBaseProductName(tt.productName)
			if got != tt.want {
				t.Errorf("extractBaseProductName(%q) = %q, want %q", tt.productName, got, tt.want)
			}
		})
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		name          string
		uptimeSeconds uint64
		want          string
	}{
		{
			name:          "zero uptime",
			uptimeSeconds: 0,
			want:          "0 minutes",
		},
		{
			name:          "30 seconds rounds to 0 minutes",
			uptimeSeconds: 30,
			want:          "0 minutes",
		},
		{
			name:          "5 minutes",
			uptimeSeconds: 300,
			want:          "5 minutes",
		},
		{
			name:          "59 minutes",
			uptimeSeconds: 59 * 60,
			want:          "59 minutes",
		},
		{
			name:          "1 hour 0 minutes",
			uptimeSeconds: 3600,
			want:          "1 hours, 0 minutes",
		},
		{
			name:          "2 hours 30 minutes",
			uptimeSeconds: 2*3600 + 30*60,
			want:          "2 hours, 30 minutes",
		},
		{
			name:          "1 day 0 hours 0 minutes",
			uptimeSeconds: 86400,
			want:          "1 days, 0 hours, 0 minutes",
		},
		{
			name:          "3 days 5 hours 42 minutes",
			uptimeSeconds: 3*86400 + 5*3600 + 42*60,
			want:          "3 days, 5 hours, 42 minutes",
		},
		{
			name:          "30 days",
			uptimeSeconds: 30 * 86400,
			want:          "30 days, 0 hours, 0 minutes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUptime(tt.uptimeSeconds)
			if got != tt.want {
				t.Errorf("FormatUptime(%d) = %q, want %q", tt.uptimeSeconds, got, tt.want)
			}
		})
	}
}

func TestGetSELinuxStatus(t *testing.T) {
	status := getSELinuxStatus()
	if status != "disabled" {
		t.Errorf("getSELinuxStatus() = %q, want %q", status, "disabled")
	}
}

func TestGetLoadAverage(t *testing.T) {
	avg := getLoadAverage()
	if len(avg) != 3 {
		t.Fatalf("getLoadAverage() returned %d elements, want 3", len(avg))
	}
	for i, v := range avg {
		if v != 0.0 {
			t.Errorf("getLoadAverage()[%d] = %f, want 0.0", i, v)
		}
	}
}

// TestDetectOS_Registry tests that DetectOS reads from the registry on a real
// Windows system. This is an integration test — it verifies the registry path
// is accessible and returns non-empty values.
func TestDetectOS_Registry(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	d := New(logger)
	osType, osVersion, err := d.DetectOS()
	if err != nil {
		t.Fatalf("DetectOS() returned error: %v", err)
	}

	if osType == "" {
		t.Error("DetectOS() returned empty osType")
	}
	if osVersion == "" || osVersion == "Unknown" {
		t.Error("DetectOS() returned empty or Unknown osVersion")
	}

	t.Logf("DetectOS() → osType=%q, osVersion=%q", osType, osVersion)
}

// TestGetKernelVersion_Registry tests that GetKernelVersion returns a valid
// build string from the registry.
func TestGetKernelVersion_Registry(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	d := New(logger)
	version := d.GetKernelVersion()

	if version == "" || version == "Unknown" {
		t.Error("GetKernelVersion() returned empty or Unknown")
	}

	// Should start with "10.0."
	if len(version) < 5 || version[:5] != "10.0." {
		t.Errorf("GetKernelVersion() = %q, expected to start with '10.0.'", version)
	}

	t.Logf("GetKernelVersion() → %q", version)
}

// TestGetLatestInstalledKernel verifies it returns the same as GetKernelVersion
func TestGetLatestInstalledKernel(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	d := New(logger)
	kernel := d.GetKernelVersion()
	latest := d.GetLatestInstalledKernel()

	if kernel != latest {
		t.Errorf("GetLatestInstalledKernel() = %q, want same as GetKernelVersion() = %q", latest, kernel)
	}
}

// TestReadNTVersionFromRegistry tests the registry reading helper directly.
func TestReadNTVersionFromRegistry(t *testing.T) {
	productName, displayVersion, currentBuild, err := readNTVersionFromRegistry()
	if err != nil {
		t.Fatalf("readNTVersionFromRegistry() error: %v", err)
	}

	if productName == "" {
		t.Error("ProductName is empty")
	}
	if currentBuild == "" {
		t.Error("CurrentBuild is empty")
	}

	t.Logf("ProductName=%q, DisplayVersion=%q, CurrentBuild=%q", productName, displayVersion, currentBuild)
}

// TestGetSystemInfo verifies the assembled SystemInfo struct has all fields populated.
func TestGetSystemInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	d := New(logger)
	info := d.GetSystemInfo()

	if info.KernelVersion == "" || info.KernelVersion == "Unknown" {
		t.Error("SystemInfo.KernelVersion is empty or Unknown")
	}
	if info.SELinuxStatus != "disabled" {
		t.Errorf("SystemInfo.SELinuxStatus = %q, want %q", info.SELinuxStatus, "disabled")
	}
	if info.SystemUptime == "" || info.SystemUptime == "Unknown" {
		t.Error("SystemInfo.SystemUptime is empty or Unknown")
	}
	if len(info.LoadAverage) != 3 {
		t.Errorf("SystemInfo.LoadAverage has %d elements, want 3", len(info.LoadAverage))
	}

	t.Logf("SystemInfo: kernel=%q, selinux=%q, uptime=%q, load=%v",
		info.KernelVersion, info.SELinuxStatus, info.SystemUptime, info.LoadAverage)
}
