package constants

// OS type constants
const (
	OSTypeWindows = "windows"
)

// Architecture constants
const (
	ArchX86_64  = "x86_64"
	ArchAMD64   = "amd64"
	ArchARM64   = "arm64"
	ArchAARCH64 = "aarch64"
	ArchUnknown = "arch_unknown"
)

// Network interface types
const (
	NetTypeEthernet = "ethernet"
	NetTypeWiFi     = "wifi"
	NetTypeBridge   = "bridge"
	NetTypeVirtual  = "virtual"
	NetTypeLoopback = "loopback"
	NetTypeUnknown  = "unknown"
)

// IP address families
const (
	IPFamilyIPv4 = "inet"
	IPFamilyIPv6 = "inet6"
)

// Repository type constants
const (
	RepoTypeWindowsUpdate = "windows-update"
)

// Log level constants
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Common error messages
const (
	ErrUnknownValue = "Unknown"
)
