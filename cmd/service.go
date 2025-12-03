package cmd

import (
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Service management commands",
	Long: `Commands for managing installed services.

Available commands:
  upgrade   Upgrade a service to a newer version`,
	Aliases: []string{"svc"},
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}
