package network

import (
	"net"
	"testing"

	"patchmon-agent/internal/constants"

	"github.com/sirupsen/logrus"
)

// TestRunPowerShell verifies the PowerShell helper can execute a simple command
func TestRunPowerShell(t *testing.T) {
	output, err := runPowerShell("Write-Output 'hello'")
	if err != nil {
		t.Skipf("PowerShell not available: %v", err)
	}
	if output != "hello" {
		t.Errorf("expected 'hello', got %q", output)
	}
}

// TestRunPowerShellEmpty verifies empty output handling
func TestRunPowerShellEmpty(t *testing.T) {
	output, err := runPowerShell("Write-Output ''")
	if err != nil {
		t.Skipf("PowerShell not available: %v", err)
	}
	if output != "" {
		t.Errorf("expected empty string, got %q", output)
	}
}

// TestParseLinkSpeed tests conversion of Windows link speed strings to Mbps
func TestParseLinkSpeed(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"1 Gbps", 1000},
		{"100 Mbps", 100},
		{"10 Gbps", 10000},
		{"2.5 Gbps", 2500},
		{"10 Mbps", 10},
		{"1000 Mbps", 1000},
		{"5 Gbps", 5000},
		{"", -1},
		{"unknown", -1},
		{"  1 Gbps  ", 1000}, // with whitespace
		{"100 mbps", 100},    // lowercase
		{"1 GBPS", 1000},     // uppercase
		{"100 Kbps", 0},      // kbps (rounds to 0)
		{"1000 Kbps", 1},     // kbps
		{"1000000 bps", 1},   // bps
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLinkSpeed(tt.input)
			if result != tt.expected {
				t.Errorf("parseLinkSpeed(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestDetectInterfaceType tests Windows interface type detection from names
func TestDetectInterfaceType(t *testing.T) {
	emptyMap := make(map[string]netAdapterInfo)

	tests := []struct {
		name     string
		expected string
	}{
		{"Wi-Fi", constants.NetTypeWiFi},
		{"WiFi", constants.NetTypeWiFi},
		{"Wireless Network Connection", constants.NetTypeWiFi},
		{"WLAN", constants.NetTypeWiFi},
		{"Ethernet", constants.NetTypeEthernet},
		{"Ethernet 2", constants.NetTypeEthernet},
		{"vEthernet (Default Switch)", constants.NetTypeVirtual},
		{"vEthernet (WSL)", constants.NetTypeVirtual},
		{"Hyper-V Virtual Ethernet Adapter", constants.NetTypeVirtual},
		{"Bluetooth Network Connection", constants.NetTypeUnknown},
		{"VMware Network Adapter VMnet8", constants.NetTypeVirtual},
		{"VirtualBox Host-Only Network", constants.NetTypeVirtual},
		{"VPN Client", constants.NetTypeVirtual},
		{"Local Area Connection", constants.NetTypeEthernet}, // default fallback
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectInterfaceType(tt.name, emptyMap)
			if result != tt.expected {
				t.Errorf("detectInterfaceType(%q) = %q, want %q", tt.name, result, tt.expected)
			}
		})
	}
}

// TestDetectInterfaceTypeWithAdapterInfo tests type detection using PowerShell adapter data
func TestDetectInterfaceTypeWithAdapterInfo(t *testing.T) {
	adapterMap := map[string]netAdapterInfo{
		"Ethernet": {
			Name:                 "Ethernet",
			InterfaceDescription: "Intel(R) Ethernet Connection I219-V",
			MediaType:            "802.3",
		},
		"Wi-Fi": {
			Name:                 "Wi-Fi",
			InterfaceDescription: "Intel(R) Wi-Fi 6 AX201 160MHz",
			MediaType:            "Native 802.11",
		},
		"vEthernet (WSL)": {
			Name:                 "vEthernet (WSL)",
			InterfaceDescription: "Hyper-V Virtual Ethernet Adapter",
			MediaType:            "",
		},
	}

	tests := []struct {
		name     string
		expected string
	}{
		{"Ethernet", constants.NetTypeEthernet},
		{"Wi-Fi", constants.NetTypeWiFi},
		{"vEthernet (WSL)", constants.NetTypeVirtual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectInterfaceType(tt.name, adapterMap)
			if result != tt.expected {
				t.Errorf("detectInterfaceType(%q) = %q, want %q", tt.name, result, tt.expected)
			}
		})
	}
}

// TestIsValidIP tests IP address validation
func TestIsValidIP(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"fe80::1", true},
		{"2001:db8::1", true},
		{"", false},
		{"not-an-ip", false},
		{"192.168.1", false},
		{"192.168.1.256", false},
		{"abc.def.ghi.jkl", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidIP(tt.input)
			if result != tt.expected {
				t.Errorf("isValidIP(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseDNSOutput tests parsing of DNS server output
func TestParseDNSOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single server",
			input:    "8.8.8.8",
			expected: []string{"8.8.8.8"},
		},
		{
			name:     "multiple servers",
			input:    "8.8.8.8\n8.8.4.4\n1.1.1.1",
			expected: []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"},
		},
		{
			name:     "with duplicates",
			input:    "8.8.8.8\n8.8.4.4\n8.8.8.8",
			expected: []string{"8.8.8.8", "8.8.4.4"},
		},
		{
			name:     "with empty lines",
			input:    "8.8.8.8\n\n8.8.4.4\n",
			expected: []string{"8.8.8.8", "8.8.4.4"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "whitespace only",
			input:    "  \n  \n  ",
			expected: []string{},
		},
		{
			name:     "with CRLF",
			input:    "8.8.8.8\r\n8.8.4.4\r\n",
			expected: []string{"8.8.8.8", "8.8.4.4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDNSOutput(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("parseDNSOutput() returned %d servers, want %d: got %v", len(result), len(tt.expected), result)
			}
			for i, s := range result {
				if s != tt.expected[i] {
					t.Errorf("parseDNSOutput()[%d] = %q, want %q", i, s, tt.expected[i])
				}
			}
		})
	}
}

// TestGetGatewayIPFormat validates that gateway IP is a valid format (integration test)
func TestGetGatewayIPFormat(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	m := New(logger)

	gateway := m.getGatewayIP()
	if gateway == "" {
		t.Skip("No default gateway found (may not have network connectivity)")
	}

	if net.ParseIP(gateway) == nil {
		t.Errorf("getGatewayIP() returned invalid IP: %q", gateway)
	}
}

// TestGetDNSServersFormat validates DNS server format (integration test)
func TestGetDNSServersFormat(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	m := New(logger)

	servers := m.getDNSServers()
	if len(servers) == 0 {
		t.Skip("No DNS servers found (may not have network connectivity)")
	}

	for _, server := range servers {
		if net.ParseIP(server) == nil {
			t.Errorf("getDNSServers() returned invalid IP: %q", server)
		}
	}
}

// TestGetNetworkInfo is an integration test that verifies GetNetworkInfo returns non-empty results
func TestGetNetworkInfo(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	m := New(logger)

	info := m.GetNetworkInfo()

	// On a real Windows machine with network, we expect at least some data
	if info.GatewayIP == "" {
		t.Log("Warning: No gateway IP found")
	} else {
		if net.ParseIP(info.GatewayIP) == nil {
			t.Errorf("GatewayIP is not a valid IP: %q", info.GatewayIP)
		}
	}

	// DNS servers should be a non-nil slice
	if info.DNSServers == nil {
		t.Error("DNSServers should not be nil")
	}

	// Validate DNS server format
	for _, server := range info.DNSServers {
		if net.ParseIP(server) == nil {
			t.Errorf("DNS server is not a valid IP: %q", server)
		}
	}

	// We should have at least one network interface on any machine
	if len(info.NetworkInterfaces) == 0 {
		t.Log("Warning: No network interfaces found")
	}

	// Validate interface fields
	validTypes := map[string]bool{
		constants.NetTypeEthernet: true,
		constants.NetTypeWiFi:     true,
		constants.NetTypeBridge:   true,
		constants.NetTypeVirtual:  true,
		constants.NetTypeUnknown:  true,
	}
	validStatuses := map[string]bool{"up": true, "down": true}

	for _, iface := range info.NetworkInterfaces {
		if iface.Name == "" {
			t.Error("Interface name should not be empty")
		}
		if !validTypes[iface.Type] {
			t.Errorf("Interface %q has invalid type: %q", iface.Name, iface.Type)
		}
		if !validStatuses[iface.Status] {
			t.Errorf("Interface %q has invalid status: %q", iface.Name, iface.Status)
		}

		// Validate addresses
		for _, addr := range iface.Addresses {
			if net.ParseIP(addr.Address) == nil {
				t.Errorf("Interface %q has invalid address: %q", iface.Name, addr.Address)
			}
			if addr.Family != constants.IPFamilyIPv4 && addr.Family != constants.IPFamilyIPv6 {
				t.Errorf("Interface %q address %q has invalid family: %q", iface.Name, addr.Address, addr.Family)
			}
		}
	}
}

// TestGetNetworkInfoDNSNeverNil ensures DNSServers is never nil
func TestGetNetworkInfoDNSNeverNil(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	m := New(logger)

	info := m.GetNetworkInfo()
	if info.DNSServers == nil {
		t.Error("DNSServers should be an empty slice, not nil")
	}
}
