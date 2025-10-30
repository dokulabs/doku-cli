package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed services",
	Long:  "List all installed services with their status, versions, and resources",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("📋 Installed Services:")
		fmt.Println("⚠️  This command is not yet implemented")
		// TODO: Implement list logic
	},
}

func init() {
	rootCmd.AddCommand(listCmd)

	// TODO: Add flags
	// listCmd.Flags().String("service", "", "Filter by service type")
	// listCmd.Flags().Bool("all", false, "Show all instances including stopped")
}
