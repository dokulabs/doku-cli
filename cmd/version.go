package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Doku",
	Long:  "Print the version number, commit hash, and build date of Doku CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Doku CLI\n")
		fmt.Printf("  Version:    %s\n", version)
		fmt.Printf("  Commit:     %s\n", commit)
		fmt.Printf("  Build Date: %s\n", date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
