package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"patchmon-agent/internal/constants"
	"patchmon-agent/pkg/models"
)

// Manager handles network information collection using PowerShell and standard library
type Manager struct {
	logger *logrus.Logger
}

// New creates a new network manager
func New(logger *logrus.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// GetNetworkInfo collects network information
func (m *Manager) GetNetworkInfo() models.NetworkInfo {
	info := models.NetworkInfo{
		GatewayIP:         m.getGatewayIP(),
		DNSServers:        m.getDNSServers(),
		NetworkInterfaces: m.getNetworkInterfaces(),
	}

	m.logger.WithFields(logrus.Fields{
		"gateway":     info.GatewayIP,
		"dns_servers": len(info.DNSServers),
		"interfaces":  len(info.NetworkInterfaces),
	}).Debug("Collected gateway, DNS, and interface information")

	return info
}

// runPowerShell executes a PowerShell command and returns trimmed output
func runPowerShell(command string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", command)
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

// getGatewayIP gets the default gateway IP using PowerShell, with ipconfig fallback
func (m *Manager) getGatewayIP() string {
	// Primary: PowerShell Get-NetRoute
	psCmd := "(Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Select-Object -First 1).NextHop"
	output, err := runPowerShell(psCmd)
	if err == nil && output != "" && isValidIP(output) {
		return output
	}
	if err != nil {
		m.logger.WithError(err).Debug("PowerShell Get-NetRoute failed, trying ipconfig fallback")
	}

	// Fallback: parse ipconfig output
	return m.getGatewayFromIPConfig()
}

// getGatewayFromIPConfig parses ipconfig output to find the default gateway
func (m *Manager) getGatewayFromIPConfig() string {
	cmd := exec.Command("ipconfig")
	output, err := cmd.Output()
	if err != nil {
		m.logger.WithError(err).Warn("Failed to run ipconfig")
		return ""
	}

	// Look for "Default Gateway" lines with an IP address
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Default Gateway") || strings.Contains(line, "Passerelle par") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				gateway := strings.TrimSpace(parts[1])
				if gateway != "" && isValidIP(gateway) {
					return gateway
				}
			}
		}
	}

	return ""
}

// getDNSServers gets the configured DNS servers using PowerShell, with ipconfig fallback
func (m *Manager) getDNSServers() []string {
	// Initialize as empty slice (not nil) to ensure JSON marshals as [] instead of null
	servers := []string{}

	// Primary: PowerShell Get-DnsClientServerAddress
	psCmd := "Get-DnsClientServerAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty ServerAddresses | Select-Object -Unique"
	output, err := runPowerShell(psCmd)
	if err == nil && output != "" {
		servers = parseDNSOutput(output)
		if len(servers) > 0 {
			return servers
		}
	}
	if err != nil {
		m.logger.WithError(err).Debug("PowerShell Get-DnsClientServerAddress failed, trying ipconfig fallback")
	}

	// Fallback: parse ipconfig /all
	return m.getDNSFromIPConfig()
}

// parseDNSOutput parses newline-separated DNS server addresses
func parseDNSOutput(output string) []string {
	servers := []string{}
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		addr := strings.TrimSpace(line)
		if addr != "" && isValidIP(addr) && !seen[addr] {
			servers = append(servers, addr)
			seen[addr] = true
		}
	}
	return servers
}

// getDNSFromIPConfig parses ipconfig /all output to find DNS servers
func (m *Manager) getDNSFromIPConfig() []string {
	servers := []string{}
	cmd := exec.Command("ipconfig", "/all")
	output, err := cmd.Output()
	if err != nil {
		m.logger.WithError(err).Warn("Failed to run ipconfig /all")
		return servers
	}

	seen := make(map[string]bool)
	inDNS := false
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(line, "DNS Servers") || strings.Contains(line, "Serveurs DNS") {
			inDNS = true
			// Extract IP from this line (after the colon)
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				addr := strings.TrimSpace(parts[1])
				if addr != "" && isValidIP(addr) && !seen[addr] {
					servers = append(servers, addr)
					seen[addr] = true
				}
			}
			continue
		}

		// Continuation lines for DNS servers (indented, no label)
		if inDNS {
			if trimmed == "" || strings.Contains(trimmed, ":") && !isValidIP(strings.TrimSpace(trimmed)) {
				inDNS = false
				continue
			}
			addr := strings.TrimSpace(trimmed)
			if isValidIP(addr) && !seen[addr] {
				servers = append(servers, addr)
				seen[addr] = true
			}
		}
	}

	return servers
}

// netAdapterInfo holds JSON output from Get-NetAdapter
type netAdapterInfo struct {
	Name                 string `json:"Name"`
	InterfaceDescription string `json:"InterfaceDescription"`
	MediaType            string `json:"MediaType"`
	Status               string `json:"Status"`
	LinkSpeed            string `json:"LinkSpeed"`
	MacAddress           string `json:"MacAddress"`
	FullDuplex           *bool  `json:"FullDuplex"`
}

// getNetworkInterfaces gets network interface information using standard library + PowerShell enrichment
func (m *Manager) getNetworkInterfaces() []models.NetworkInterface {
	interfaces, err := net.Interfaces()
	if err != nil {
		m.logger.WithError(err).Warn("Failed to get network interfaces")
		return []models.NetworkInterface{}
	}

	// Get enriched adapter info from PowerShell
	adapterMap := m.getAdapterInfo()

	var result []models.NetworkInterface

	for _, iface := range interfaces {
		// Skip loopback interface
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Get IP addresses for this interface
		var addresses []models.NetworkAddress

		addrs, err := iface.Addrs()
		if err != nil {
			m.logger.WithError(err).WithField("interface", iface.Name).Warn("Failed to get addresses for interface")
			continue
		}

		// Get gateways for this interface (separate for IPv4 and IPv6)
		ipv4Gateway := m.getInterfaceGateway(iface.Name, false)
		ipv6Gateway := m.getInterfaceGateway(iface.Name, true)

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				var family string
				var gateway string

				if ipnet.IP.To4() != nil {
					family = constants.IPFamilyIPv4
					gateway = ipv4Gateway
				} else {
					family = constants.IPFamilyIPv6
					// Link-local addresses don't have gateways
					if ipnet.IP.IsLinkLocalUnicast() {
						gateway = ""
					} else {
						gateway = ipv6Gateway
					}
				}

				// Calculate netmask in CIDR notation
				ones, _ := ipnet.Mask.Size()
				netmask := fmt.Sprintf("/%d", ones)

				addresses = append(addresses, models.NetworkAddress{
					Address: ipnet.IP.String(),
					Family:  family,
					Netmask: netmask,
					Gateway: gateway,
				})
			}
		}

		// Include interface even if it has no addresses (to show MAC, status, etc.)
		if len(addresses) > 0 || iface.Flags&net.FlagUp != 0 {
			// Determine interface type from Windows adapter info or name heuristics
			interfaceType := detectInterfaceType(iface.Name, adapterMap)

			// Get MAC address
			macAddress := ""
			if len(iface.HardwareAddr) > 0 {
				macAddress = iface.HardwareAddr.String()
			}

			// Get status
			status := "down"
			if iface.Flags&net.FlagUp != 0 {
				status = "up"
			}

			// Get link speed and duplex from PowerShell adapter info
			linkSpeed, duplex := m.getLinkSpeedAndDuplex(iface.Name, adapterMap)

			result = append(result, models.NetworkInterface{
				Name:       iface.Name,
				Type:       interfaceType,
				MACAddress: macAddress,
				MTU:        iface.MTU,
				Status:     status,
				LinkSpeed:  linkSpeed,
				Duplex:     duplex,
				Addresses:  addresses,
			})
		}
	}

	return result
}

// getAdapterInfo retrieves adapter details from PowerShell Get-NetAdapter
func (m *Manager) getAdapterInfo() map[string]netAdapterInfo {
	adapterMap := make(map[string]netAdapterInfo)

	psCmd := "Get-NetAdapter -ErrorAction SilentlyContinue | Select-Object Name, InterfaceDescription, MediaType, Status, LinkSpeed, MacAddress, FullDuplex | ConvertTo-Json"
	output, err := runPowerShell(psCmd)
	if err != nil {
		m.logger.WithError(err).Debug("Failed to get adapter info from PowerShell")
		return adapterMap
	}

	if output == "" {
		return adapterMap
	}

	// PowerShell returns a single object (not array) when there's only one adapter
	// Try array first, then single object
	var adapters []netAdapterInfo
	if err := json.Unmarshal([]byte(output), &adapters); err != nil {
		var single netAdapterInfo
		if err2 := json.Unmarshal([]byte(output), &single); err2 != nil {
			m.logger.WithError(err2).Debug("Failed to parse adapter JSON")
			return adapterMap
		}
		adapters = []netAdapterInfo{single}
	}

	for _, adapter := range adapters {
		adapterMap[adapter.Name] = adapter
	}

	return adapterMap
}

// detectInterfaceType determines the interface type from Windows adapter info or name heuristics
func detectInterfaceType(name string, adapterMap map[string]netAdapterInfo) string {
	nameLower := strings.ToLower(name)

	// Check PowerShell adapter info first
	if adapter, ok := adapterMap[name]; ok {
		descLower := strings.ToLower(adapter.InterfaceDescription)
		mediaLower := strings.ToLower(adapter.MediaType)

		// Check media type
		if strings.Contains(mediaLower, "802.3") || strings.Contains(mediaLower, "ethernet") {
			return constants.NetTypeEthernet
		}
		if strings.Contains(mediaLower, "802.11") || strings.Contains(mediaLower, "wireless") || strings.Contains(mediaLower, "native 802.11") {
			return constants.NetTypeWiFi
		}

		// Check description for known patterns
		if strings.Contains(descLower, "wi-fi") || strings.Contains(descLower, "wifi") ||
			strings.Contains(descLower, "wireless") || strings.Contains(descLower, "wlan") {
			return constants.NetTypeWiFi
		}
		if strings.Contains(descLower, "hyper-v") || strings.Contains(descLower, "virtual") ||
			strings.Contains(descLower, "vmware") || strings.Contains(descLower, "virtualbox") ||
			strings.Contains(descLower, "vpn") || strings.Contains(descLower, "tap-") {
			return constants.NetTypeVirtual
		}
		if strings.Contains(descLower, "bluetooth") {
			return constants.NetTypeUnknown
		}
	}

	// Fallback: name-based heuristics for common Windows interface names
	// Order matters: check more specific patterns before generic ones
	if strings.Contains(nameLower, "wi-fi") || strings.Contains(nameLower, "wifi") ||
		strings.Contains(nameLower, "wireless") || strings.Contains(nameLower, "wlan") {
		return constants.NetTypeWiFi
	}
	if strings.Contains(nameLower, "bluetooth") {
		return constants.NetTypeUnknown
	}
	// Check virtual patterns before "ethernet" since "vethernet" contains "ethernet"
	if strings.Contains(nameLower, "vethernet") || strings.Contains(nameLower, "hyper-v") ||
		strings.Contains(nameLower, "vmware") || strings.Contains(nameLower, "virtualbox") ||
		strings.Contains(nameLower, "vpn") || strings.Contains(nameLower, "virtual") {
		return constants.NetTypeVirtual
	}
	if strings.Contains(nameLower, "ethernet") {
		return constants.NetTypeEthernet
	}

	// Default to ethernet for physical-looking interfaces
	return constants.NetTypeEthernet
}

// getInterfaceGateway gets the gateway IP for a specific interface using PowerShell
func (m *Manager) getInterfaceGateway(interfaceName string, ipv6 bool) string {
	var prefix string
	if ipv6 {
		prefix = "::/0"
	} else {
		prefix = "0.0.0.0/0"
	}

	// Escape single quotes in interface name for PowerShell
	escapedName := strings.ReplaceAll(interfaceName, "'", "''")
	psCmd := fmt.Sprintf(
		"(Get-NetRoute -InterfaceAlias '%s' -DestinationPrefix '%s' -ErrorAction SilentlyContinue | Select-Object -First 1).NextHop",
		escapedName, prefix,
	)

	output, err := runPowerShell(psCmd)
	if err != nil {
		m.logger.WithError(err).WithField("interface", interfaceName).Debug("Failed to get interface gateway via PowerShell")
		return ""
	}

	if output != "" && isValidIP(output) {
		return output
	}

	return ""
}

// getLinkSpeedAndDuplex gets the link speed (in Mbps) and duplex mode for an interface
func (m *Manager) getLinkSpeedAndDuplex(interfaceName string, adapterMap map[string]netAdapterInfo) (int, string) {
	adapter, ok := adapterMap[interfaceName]
	if !ok {
		return -1, ""
	}

	// Parse link speed string (e.g., "1 Gbps", "100 Mbps", "10 Gbps", "2.5 Gbps")
	linkSpeed := parseLinkSpeed(adapter.LinkSpeed)

	// Determine duplex
	duplex := ""
	if adapter.FullDuplex != nil {
		if *adapter.FullDuplex {
			duplex = "full"
		} else {
			duplex = "half"
		}
	}

	return linkSpeed, duplex
}

// parseLinkSpeed converts a Windows link speed string to Mbps
// Examples: "1 Gbps" → 1000, "100 Mbps" → 100, "10 Gbps" → 10000, "2.5 Gbps" → 2500
func parseLinkSpeed(speedStr string) int {
	if speedStr == "" {
		return -1
	}

	speedStr = strings.TrimSpace(speedStr)

	// Match patterns like "100 Mbps", "1 Gbps", "2.5 Gbps", "10 Gbps"
	re := regexp.MustCompile(`(?i)^([\d.]+)\s*(gbps|mbps|kbps|bps)$`)
	matches := re.FindStringSubmatch(speedStr)
	if len(matches) != 3 {
		return -1
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return -1
	}

	unit := strings.ToLower(matches[2])
	switch unit {
	case "gbps":
		return int(value * 1000)
	case "mbps":
		return int(value)
	case "kbps":
		return int(value / 1000)
	case "bps":
		return int(value / 1000000)
	}

	return -1
}

// isValidIP checks if a string is a valid IPv4 or IPv6 address
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}
