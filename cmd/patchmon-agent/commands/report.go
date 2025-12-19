package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"patchmon-agent/internal/client"
	"patchmon-agent/internal/hardware"
	"patchmon-agent/internal/integrations"
	"patchmon-agent/internal/integrations/docker"
	"patchmon-agent/internal/network"
	"patchmon-agent/internal/packages"
	"patchmon-agent/internal/repositories"
	"patchmon-agent/internal/system"
	"patchmon-agent/internal/version"
	"patchmon-agent/pkg/models"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var reportJson bool

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Report system and package information to server",
	Long:  "Collect and report system, package, and repository information to the PatchMon server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkRoot(); err != nil {
			return err
		}

		return sendReport(reportJson)
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportJson, "json", false, "Output the JSON report payload to stdout instead of sending to server")
}

func sendReport(outputJson bool) error {
	// Start tracking execution time
	startTime := time.Now()
	logger.Debug("Starting report process")

	// Load API credentials only if we're sending the report (not just outputting JSON)
	if !outputJson {
		logger.Debug("Loading API credentials")
		if err := cfgManager.LoadCredentials(); err != nil {
			logger.WithError(err).Debug("Failed to load credentials")
			return err
		}
	}

	// Initialise managers
	systemDetector := system.New(logger)
	packageMgr := packages.New(logger)
	repoMgr := repositories.New(logger)
	hardwareMgr := hardware.New(logger)
	networkMgr := network.New(logger)

	// Detect OS
	logger.Info("Detecting operating system...")
	osType, osVersion, err := systemDetector.DetectOS()
	if err != nil {
		return fmt.Errorf("failed to detect OS: %w", err)
	}
	logger.WithFields(logrus.Fields{
		"osType":    osType,
		"osVersion": osVersion,
	}).Info("Detected OS")

	// Get system information
	logger.Info("Collecting system information...")
	hostname, err := systemDetector.GetHostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	architecture := systemDetector.GetArchitecture()
	systemInfo := systemDetector.GetSystemInfo()
	ipAddress := systemDetector.GetIPAddress()

	// Get hardware information
	logger.Info("Collecting hardware information...")
	hardwareInfo := hardwareMgr.GetHardwareInfo()

	// Get network information
	logger.Info("Collecting network information...")
	networkInfo := networkMgr.GetNetworkInfo()
	// Ensure DNSServers is never nil (should be empty slice, not nil)
	if networkInfo.DNSServers == nil {
		networkInfo.DNSServers = []string{}
	}

	// Check if reboot is required and get installed kernel
	logger.Info("Checking reboot status...")
	needsReboot, rebootReason := systemDetector.CheckRebootRequired()
	installedKernel := systemDetector.GetLatestInstalledKernel()
	logger.WithFields(logrus.Fields{
		"needs_reboot":        needsReboot,
		"reason":              rebootReason,
		"installed_kernel":    installedKernel,
		"running_kernel":      systemInfo.KernelVersion,
	}).Info("Reboot status check completed")

	// Get package information
	logger.Info("Collecting package information...")
	packageList, err := packageMgr.GetPackages()
	if err != nil {
		return fmt.Errorf("failed to get packages: %w", err)
	}
	// Ensure packageList is never nil (should be empty slice, not nil)
	if packageList == nil {
		packageList = []models.Package{}
	}

	// Count packages for debug logging
	needsUpdateCount := 0
	securityUpdateCount := 0
	for _, pkg := range packageList {
		if pkg.NeedsUpdate {
			needsUpdateCount++
		}
		if pkg.IsSecurityUpdate {
			securityUpdateCount++
		}
	}
	logger.WithField("count", len(packageList)).Info("Found packages")
	for _, pkg := range packageList {
		updateMsg := ""
		if pkg.NeedsUpdate {
			updateMsg = "update available"
		} else {
			updateMsg = "latest"
		}
		logger.WithFields(logrus.Fields{
			"name":    pkg.Name,
			"version": pkg.CurrentVersion,
			"status":  updateMsg,
		}).Debug("Package info")
	}
	logger.WithFields(logrus.Fields{
		"total_updates":    needsUpdateCount,
		"security_updates": securityUpdateCount,
	}).Debug("Package summary")

	// Get repository information
	logger.Info("Collecting repository information...")
	repoList, err := repoMgr.GetRepositories()
	if err != nil {
		logger.WithError(err).Warn("Failed to get repositories")
		repoList = []models.Repository{}
	}
	logger.WithField("count", len(repoList)).Info("Found repositories")
	for _, repo := range repoList {
		logger.WithFields(logrus.Fields{
			"name":    repo.Name,
			"type":    repo.RepoType,
			"url":     repo.URL,
			"enabled": repo.IsEnabled,
		}).Debug("Repository info")
	}

	// Calculate execution time (in seconds, with millisecond precision)
	executionTime := time.Since(startTime).Seconds()
	logger.WithField("execution_time_seconds", executionTime).Debug("Data collection completed")

	// Create payload
	payload := &models.ReportPayload{
		Packages:          packageList,
		Repositories:      repoList,
		OSType:            osType,
		OSVersion:         osVersion,
		Hostname:          hostname,
		IP:                ipAddress,
		Architecture:      architecture,
		AgentVersion:      version.Version,
		MachineID:             systemDetector.GetMachineID(),
		KernelVersion:         systemInfo.KernelVersion,
		InstalledKernelVersion: installedKernel,
		SELinuxStatus:         systemInfo.SELinuxStatus,
		SystemUptime:      systemInfo.SystemUptime,
		LoadAverage:       systemInfo.LoadAverage,
		CPUModel:          hardwareInfo.CPUModel,
		CPUCores:          hardwareInfo.CPUCores,
		RAMInstalled:      hardwareInfo.RAMInstalled,
		SwapSize:          hardwareInfo.SwapSize,
		DiskDetails:       hardwareInfo.DiskDetails,
		GatewayIP:         networkInfo.GatewayIP,
		DNSServers:        networkInfo.DNSServers,
		NetworkInterfaces: networkInfo.NetworkInterfaces,
		ExecutionTime:     executionTime,
		NeedsReboot:       needsReboot,
		RebootReason:      rebootReason,
	}

	// If --report-json flag is set, output JSON and exit
	if outputJson {
		jsonData, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintf(os.Stdout, "%s\n", jsonData)
		return nil
	}

	// Send report
	logger.Info("Sending report to PatchMon server...")
	httpClient := client.New(cfgManager, logger)
	ctx := context.Background()
	response, err := httpClient.SendUpdate(ctx, payload)
	if err != nil {
		return fmt.Errorf("failed to send report: %w", err)
	}

	logger.Info("Report sent successfully")
	logger.WithField("count", response.PackagesProcessed).Info("Processed packages")

	// Handle agent auto-update (server-initiated)
	if response.AutoUpdate != nil && response.AutoUpdate.ShouldUpdate {
		logger.WithFields(logrus.Fields{
			"current": response.AutoUpdate.CurrentVersion,
			"latest":  response.AutoUpdate.LatestVersion,
			"message": response.AutoUpdate.Message,
		}).Info("PatchMon agent update detected")

		logger.Info("Automatically updating PatchMon agent to latest version...")
		if err := updateAgent(); err != nil {
			logger.WithError(err).Warn("PatchMon agent update failed, but data was sent successfully")
		} else {
			logger.Info("PatchMon agent update completed successfully")
			// updateAgent() will exit the process after restart, so we won't reach here
			// But if it does return, skip the update check to prevent loops
			return nil
		}
	} else {
		// Proactive update check after report (non-blocking with timeout)
		// Run in a goroutine to avoid blocking the report completion
		go func() {
			// Add a delay to prevent immediate checks after service restart
			// This gives the new process time to fully initialize
			time.Sleep(5 * time.Second)
			
			logger.Info("Checking for agent updates...")
			versionInfo, err := getServerVersionInfo()
			if err != nil {
				logger.WithError(err).Warn("Failed to check for updates after report (non-critical)")
				return
			}
			if versionInfo.HasUpdate {
				logger.WithFields(logrus.Fields{
					"current": versionInfo.CurrentVersion,
					"latest":  versionInfo.LatestVersion,
				}).Info("Update available, automatically updating...")

				if err := updateAgent(); err != nil {
					logger.WithError(err).Warn("PatchMon agent update failed, but data was sent successfully")
				} else {
					logger.Info("PatchMon agent update completed successfully")
					// updateAgent() will exit after restart, so this won't be reached
				}
			} else if versionInfo.AutoUpdateDisabled && versionInfo.LatestVersion != versionInfo.CurrentVersion {
				// Update is available but auto-update is disabled
				logger.WithFields(logrus.Fields{
					"current": versionInfo.CurrentVersion,
					"latest":  versionInfo.LatestVersion,
					"reason":  versionInfo.AutoUpdateDisabledReason,
				}).Info("New update available but auto-update is disabled")
			} else {
				logger.WithField("version", versionInfo.CurrentVersion).Info("Agent is up to date")
			}
		}()
	}

	// Collect and send integration data (Docker, etc.) separately
	// This ensures failures in integrations don't affect core system reporting
	sendIntegrationData()

	logger.Debug("Report process completed")
	return nil
}

// sendIntegrationData collects and sends data from integrations (Docker, etc.)
func sendIntegrationData() {
	logger.Debug("Starting integration data collection")

	// Create integration manager
	integrationMgr := integrations.NewManager(logger)

	// Set enabled checker to respect config.yml settings
	// Load config first to check integration status
	if err := cfgManager.LoadConfig(); err != nil {
		logger.WithError(err).Debug("Failed to load config for integration check")
	}
	integrationMgr.SetEnabledChecker(func(name string) bool {
		return cfgManager.IsIntegrationEnabled(name)
	})

	// Register available integrations
	integrationMgr.Register(docker.New(logger))
	// Future: integrationMgr.Register(proxmox.New(logger))
	// Future: integrationMgr.Register(kubernetes.New(logger))

	// Discover and collect from all available integrations
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	integrationData := integrationMgr.CollectAll(ctx)

	if len(integrationData) == 0 {
		logger.Debug("No integration data to send")
		return
	}

	// Get system info for integration payloads
	systemDetector := system.New(logger)
	hostname, _ := systemDetector.GetHostname()
	machineID := systemDetector.GetMachineID()

	// Create HTTP client
	httpClient := client.New(cfgManager, logger)

	// Send Docker data if available
	if dockerData, exists := integrationData["docker"]; exists && dockerData.Error == "" {
		sendDockerData(httpClient, dockerData, hostname, machineID)
	}

	// Future: Send other integration data here
}

// sendDockerData sends Docker integration data to server
func sendDockerData(httpClient *client.Client, integrationData *models.IntegrationData, hostname, machineID string) {
	// Extract Docker data from integration data
	dockerData, ok := integrationData.Data.(*models.DockerData)
	if !ok {
		logger.Warn("Failed to extract Docker data from integration")
		return
	}

	payload := &models.DockerPayload{
		DockerData:   *dockerData,
		Hostname:     hostname,
		MachineID:    machineID,
		AgentVersion: version.Version,
	}

	logger.WithFields(logrus.Fields{
		"containers": len(dockerData.Containers),
		"images":     len(dockerData.Images),
		"volumes":    len(dockerData.Volumes),
		"networks":   len(dockerData.Networks),
		"updates":    len(dockerData.Updates),
	}).Info("Sending Docker data to server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := httpClient.SendDockerData(ctx, payload)
	if err != nil {
		logger.WithError(err).Warn("Failed to send Docker data (will retry on next report)")
		return
	}

	logger.WithFields(logrus.Fields{
		"containers": response.ContainersReceived,
		"images":     response.ImagesReceived,
		"volumes":    response.VolumesReceived,
		"networks":   response.NetworksReceived,
		"updates":    response.UpdatesFound,
	}).Info("Docker data sent successfully")
}
