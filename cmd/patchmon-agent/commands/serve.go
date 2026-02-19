package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// serveCmd runs the agent as a long-lived service
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the agent as a Windows service (V2)",
	Long:  "Run the agent as a Windows service with async updates. This feature will be available in V2.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkAdmin(); err != nil {
			return err
		}
		fmt.Println("Windows Service mode will be available in V2.")
		fmt.Println("For now, use 'patchmon-agent report' to send a one-time report,")
		fmt.Println("or schedule it via Windows Task Scheduler.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
