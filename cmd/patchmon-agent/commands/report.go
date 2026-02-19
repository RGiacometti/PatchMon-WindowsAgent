package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"patchmon-agent/internal/client"
	"patchmon-agent/internal/hardware"
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
		if err := checkAdmin(); err != nil {
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
		"needs_reboot":     needsReboot,
		"reason":           rebootReason,
		"installed_kernel": installedKernel,
		"running_kernel":   systemInfo.KernelVersion,
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
		Packages:               packageList,
		Repositories:           repoList,
		OSType:                 osType,
		OSVersion:              osVersion,
		Hostname:               hostname,
		IP:                     ipAddress,
		Architecture:           architecture,
		AgentVersion:           version.Version,
		MachineID:              systemDetector.GetMachineID(),
		KernelVersion:          systemInfo.KernelVersion,
		InstalledKernelVersion: installedKernel,
		SELinuxStatus:          systemInfo.SELinuxStatus,
		SystemUptime:           systemInfo.SystemUptime,
		LoadAverage:            systemInfo.LoadAverage,
		CPUModel:               hardwareInfo.CPUModel,
		CPUCores:               hardwareInfo.CPUCores,
		RAMInstalled:           hardwareInfo.RAMInstalled,
		SwapSize:               hardwareInfo.SwapSize,
		DiskDetails:            hardwareInfo.DiskDetails,
		GatewayIP:              networkInfo.GatewayIP,
		DNSServers:             networkInfo.DNSServers,
		NetworkInterfaces:      networkInfo.NetworkInterfaces,
		ExecutionTime:          executionTime,
		NeedsReboot:            needsReboot,
		RebootReason:           rebootReason,
	}

	// If --report-json flag is set, output JSON and exit
	if outputJson {
		jsonData, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s\n", jsonData); err != nil {
			return fmt.Errorf("failed to write JSON output: %w", err)
		}
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
			return nil
		}
	} else {
		// Proactive update check after report (non-blocking with timeout)
		go func() {
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
				}
			} else if versionInfo.AutoUpdateDisabled && versionInfo.LatestVersion != versionInfo.CurrentVersion {
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

	logger.Debug("Report process completed")
	return nil
}
