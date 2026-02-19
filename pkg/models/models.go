package models

// Config holds the agent configuration
type Config struct {
	PatchmonServer  string          `mapstructure:"patchmon_server" json:"patchmon_server"`
	APIVersion      string          `mapstructure:"api_version" json:"api_version"`
	CredentialsFile string          `mapstructure:"credentials_file" json:"credentials_file"`
	LogFile         string          `mapstructure:"log_file" json:"log_file"`
	LogLevel        string          `mapstructure:"log_level" json:"log_level"`
	SkipSSLVerify   bool            `mapstructure:"skip_ssl_verify" json:"skip_ssl_verify"`
	UpdateInterval  int             `mapstructure:"update_interval" json:"update_interval"`
	ReportOffset    int             `mapstructure:"report_offset" json:"report_offset"`
	Integrations    map[string]bool `mapstructure:"integrations" json:"integrations"`
}

// Credentials holds API authentication credentials
type Credentials struct {
	APIID  string `mapstructure:"api_id" json:"api_id"`
	APIKey string `mapstructure:"api_key" json:"api_key"`
}

// SystemInfo holds system-level information
type SystemInfo struct {
	KernelVersion string    `json:"kernelVersion"`
	SELinuxStatus string    `json:"selinuxStatus"`
	SystemUptime  string    `json:"systemUptime"`
	LoadAverage   []float64 `json:"loadAverage"`
}

// HardwareInfo holds hardware information
type HardwareInfo struct {
	CPUModel     string     `json:"cpuModel"`
	CPUCores     int        `json:"cpuCores"`
	RAMInstalled float64    `json:"ramInstalled"`
	SwapSize     float64    `json:"swapSize"`
	DiskDetails  []DiskInfo `json:"diskDetails"`
}

// DiskInfo holds information about a single disk
type DiskInfo struct {
	Name       string `json:"name"`
	Size       string `json:"size"`
	MountPoint string `json:"mountPoint"`
}

// NetworkInfo holds network information
type NetworkInfo struct {
	GatewayIP         string             `json:"gatewayIp"`
	DNSServers        []string           `json:"dnsServers"`
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces"`
}

// NetworkInterface holds information about a single network interface
type NetworkInterface struct {
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	MACAddress string           `json:"macAddress"`
	MTU        int              `json:"mtu"`
	Status     string           `json:"status"`
	LinkSpeed  int              `json:"linkSpeed"`
	Duplex     string           `json:"duplex"`
	Addresses  []NetworkAddress `json:"addresses"`
}

// NetworkAddress holds a single IP address configuration
type NetworkAddress struct {
	Address string `json:"address"`
	Family  string `json:"family"`
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
}

// Package holds information about a single package/update
type Package struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	CurrentVersion   string `json:"currentVersion,omitempty"`
	AvailableVersion string `json:"availableVersion,omitempty"`
	NeedsUpdate      bool   `json:"needsUpdate"`
	IsSecurityUpdate bool   `json:"isSecurityUpdate"`
}

// Repository holds information about a package repository/update source
type Repository struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Distribution string `json:"distribution"`
	Components   string `json:"components"`
	RepoType     string `json:"repoType"`
	IsEnabled    bool   `json:"isEnabled"`
	IsSecure     bool   `json:"isSecure"`
}

// ReportPayload is the full payload sent to the PatchMon server
type ReportPayload struct {
	Packages               []Package          `json:"packages"`
	Repositories           []Repository       `json:"repositories"`
	OSType                 string             `json:"osType"`
	OSVersion              string             `json:"osVersion"`
	Hostname               string             `json:"hostname"`
	IP                     string             `json:"ip"`
	Architecture           string             `json:"architecture"`
	AgentVersion           string             `json:"agentVersion"`
	MachineID              string             `json:"machineId"`
	KernelVersion          string             `json:"kernelVersion"`
	InstalledKernelVersion string             `json:"installedKernelVersion"`
	SELinuxStatus          string             `json:"selinuxStatus"`
	SystemUptime           string             `json:"systemUptime"`
	LoadAverage            []float64          `json:"loadAverage"`
	CPUModel               string             `json:"cpuModel"`
	CPUCores               int                `json:"cpuCores"`
	RAMInstalled           float64            `json:"ramInstalled"`
	SwapSize               float64            `json:"swapSize"`
	DiskDetails            []DiskInfo         `json:"diskDetails"`
	GatewayIP              string             `json:"gatewayIp"`
	DNSServers             []string           `json:"dnsServers"`
	NetworkInterfaces      []NetworkInterface `json:"networkInterfaces"`
	ExecutionTime          float64            `json:"executionTime"`
	NeedsReboot            bool               `json:"needsReboot"`
	RebootReason           string             `json:"rebootReason"`
}

// PingResponse is the response from the server ping endpoint
type PingResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// AutoUpdateInfo holds server-initiated auto-update information
type AutoUpdateInfo struct {
	ShouldUpdate   bool   `json:"shouldUpdate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Message        string `json:"message"`
}

// UpdateResponse is the response from the server update endpoint
type UpdateResponse struct {
	PackagesProcessed int             `json:"packagesProcessed"`
	AutoUpdate        *AutoUpdateInfo `json:"autoUpdate,omitempty"`
}

// UpdateIntervalResponse is the response from the server update-interval endpoint
type UpdateIntervalResponse struct {
	Interval int `json:"interval"`
}

// IntegrationStatusResponse is the response from the integration status endpoint
type IntegrationStatusResponse struct {
	Integrations map[string]bool `json:"integrations"`
}
