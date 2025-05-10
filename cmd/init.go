package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Spin up a local K3s Kubernetes cluster",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Setting up your local K3s cluster...")
		// Here you will call internal/k3s/installer.go
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
