/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cluster

import (
	"github.com/dokulabs/doku/pkg"
	"github.com/spf13/cobra"
)

func NewStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start a kubernetes cluster using Docker driver",
		Long: `This command initializes a local kubernetes cluster 
using Docker as the container driver.`,
		Run: func(cmd *cobra.Command, args []string) {
			spinner := pkg.NewSpinner()
			spinner.Start("Detecting the Kubernetes...")
			if !pkg.IsDockerRunning() {
				spinner.Error("Docker is not running, Please start the docker and try again.")
			}
			manager, err := pkg.GetClusterManager(spinner)
			if err != nil {
				spinner.Error("Failed get cluster information. Please run `doku init --overwrite` to reinitialise the project.", err)
			}
			if manager.IsRunning() {
				spinner.Notice("%s cluster is already running.", manager.Name())
				spinner.StopSilently()
				return
			}

			spinner.UpdateMessage("Starting %s cluster...", manager.Name())
			err = manager.Start()
			if err != nil {
				spinner.Error("Failed starting %s cluster.", manager.Name(), err)
			}
			spinner.Stop("Started %s cluster.", manager.Name())
		},
	}
}
