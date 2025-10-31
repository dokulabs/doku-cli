package cmd

import (
	"github.com/spf13/cobra"
)

var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage the doku CLI itself",
	Long: `Commands for managing the doku CLI tool itself.

Available commands:
  upgrade - Upgrade doku to the latest version`,
}

func init() {
	rootCmd.AddCommand(selfCmd)
}
