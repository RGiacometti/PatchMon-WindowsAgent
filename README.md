# PatchMon Windows Agent

A Windows agent for [PatchMon](https://github.com/PatchMon/PatchMon) that collects Windows Update information and system data, then reports it to a PatchMon server.

This is a Windows-only fork of the [PatchMon Agent](https://github.com/PatchMon/PatchMon-agent), adapted to collect Windows-specific data while maintaining compatibility with the unmodified PatchMon server API.

## Features

- **Windows Update Collection**: Queries installed and available updates via the Windows Update Agent COM API
- **Security Update Detection**: Identifies security and critical updates via MSRC severity and update categories
- **System Information**: OS version (Windows 10/11/Server), build number, architecture, uptime
- **Hardware Information**: CPU, RAM, swap (pagefile), disk details
- **Network Information**: Interfaces, gateway, DNS servers, link speed
- **Reboot Detection**: Checks Windows registry for pending reboot indicators
- **Update Source Detection**: Identifies WSUS, Microsoft Update, or Windows Update as the update source

## Requirements

- Windows 10/11 or Windows Server 2019/2022
- Administrator privileges (required for Windows Update COM API access)
- Go 1.25+ (for building from source)

## Installation

### From Release

1. Download the latest release from [GitHub Releases](../../releases)
2. Place `patchmon-agent.exe` in `C:\Program Files\PatchMon\`
3. Create configuration directory: `C:\ProgramData\PatchMon\`
4. Create `C:\ProgramData\PatchMon\credentials.yml`:
   ```yaml
   api_id: "your-api-id"
   api_key: "your-api-key"
   ```
5. Create `C:\ProgramData\PatchMon\config.yml`:
   ```yaml
   patchmon_server: "https://your-patchmon-server.com"
   api_version: "v1"
   credentials_file: "C:\\ProgramData\\PatchMon\\credentials.yml"
   log_file: "C:\\ProgramData\\PatchMon\\logs\\patchmon-agent.log"
   log_level: "info"
   skip_ssl_verify: false
   update_interval: 60
   ```

### From Source

```bash
git clone https://github.com/your-org/PatchMon-WindowsAgent.git
cd PatchMon-WindowsAgent
make build
make install       # Copies binary to C:\Program Files\PatchMon\ (requires Admin)
make config-init   # Creates config directory and sample files (requires Admin)
```

## Usage

### Initial Setup

```powershell
# Run as Administrator — configure credentials and server URL
.\patchmon-agent.exe config set-api <API_ID> <API_KEY> <SERVER_URL>
```

Example:
```powershell
.\patchmon-agent.exe config set-api patchmon_1a2b3c4d abcd1234567890abcdef http://patchmon.example.com
```

### Send Report (to server)

```powershell
# Run as Administrator
.\patchmon-agent.exe report
```

### JSON Output (for testing, no server needed)

```powershell
# Run as Administrator
.\patchmon-agent.exe report --json
```

### Configuration

```powershell
.\patchmon-agent.exe config show
.\patchmon-agent.exe config set <key> <value>
.\patchmon-agent.exe config set-api <API_ID> <API_KEY> <SERVER_URL>
```

### Connectivity Test

```powershell
# Run as Administrator
.\patchmon-agent.exe ping
```

### Diagnostics

```powershell
.\patchmon-agent.exe diagnostics
```

### Version & Updates

```powershell
.\patchmon-agent.exe check-version
.\patchmon-agent.exe update-agent
```

## Available Commands

| Command | Description |
|---------|-------------|
| `report` | Collect and send system & package information to the PatchMon server |
| `report --json` | Output the JSON report payload to stdout instead of sending |
| `ping` | Test connectivity to the server and validate API credentials |
| `config show` | Display current configuration |
| `config set <key> <value>` | Set a configuration value |
| `config set-api <id> <key> <url>` | Configure API credentials and server URL |
| `check-version` | Check for agent updates |
| `update-agent` | Update the agent to the latest version |
| `diagnostics` | Show detailed system and agent diagnostics |

## Data Collected

| Field | Source | Example |
|-------|--------|---------|
| OS Type | Registry `ProductName` | "Windows 10", "Windows Server 2022" |
| OS Version | Registry `DisplayVersion` | "23H2", "24H2" |
| Kernel Version | Registry `CurrentBuild.UBR` | "10.0.19045.3803" |
| Packages | Windows Update COM API | KB IDs with security flags |
| Repositories | Registry (WSUS/WU config) | "Microsoft Update", "WSUS" |
| Reboot Status | Registry keys | Pending reboot indicators |
| Hardware | gopsutil | CPU, RAM, disks |
| Network | PowerShell + net.Interfaces | Gateway, DNS, interfaces |

## Configuration Files

| File | Path | Purpose |
|------|------|---------|
| Config | `C:\ProgramData\PatchMon\config.yml` | Agent configuration |
| Credentials | `C:\ProgramData\PatchMon\credentials.yml` | API authentication |
| Logs | `C:\ProgramData\PatchMon\logs\patchmon-agent.log` | Agent logs |

## Building

```bash
# Default (Windows amd64)
make build

# All architectures (amd64, arm64, 386)
make build-all

# Run tests
make test

# Run tests in short mode (skip integration tests)
make test-short

# Lint (go vet)
make lint

# Format code
make fmt

# Clean build artifacts
make clean

# Install to C:\Program Files\PatchMon\ (requires Admin)
make install

# Create config directory and sample files (requires Admin)
make config-init
```

## Logging

Logs are written to `C:\ProgramData\PatchMon\logs\patchmon-agent.log` with rotation (10 MB max, 5 backups, 14-day retention):

```
2023-09-27T10:30:00 level=info msg="Collecting package information..."
2023-09-27T10:30:01 level=info msg="Found packages" count=156
2023-09-27T10:30:02 level=info msg="Sending report to PatchMon server..."
2023-09-27T10:30:03 level=info msg="Report sent successfully"
```

Log levels: `debug`, `info`, `warn`, `error`

## Troubleshooting

### Common Issues

1. **"This command must be run as Administrator"**:
   Open PowerShell or Command Prompt as Administrator before running the agent.

2. **Credentials Not Found**:
   ```powershell
   .\patchmon-agent.exe config set-api <API_ID> <API_KEY> <SERVER_URL>
   ```

3. **Network Connectivity**:
   ```powershell
   .\patchmon-agent.exe ping
   .\patchmon-agent.exe diagnostics
   ```

4. **Windows Update COM Errors**:
   Ensure the Windows Update service is running:
   ```powershell
   Get-Service wuauserv | Start-Service
   ```

## Roadmap (V2)

- [ ] Windows Service mode (background service with periodic reporting)
- [ ] WebSocket real-time communication
- [ ] Auto-update mechanism
- [ ] MSI installer
- [ ] Task Scheduler integration

## License

[Same as original PatchMon Agent](LICENSE)
