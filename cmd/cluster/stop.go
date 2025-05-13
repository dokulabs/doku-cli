/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cluster

import (
	"github.com/dokulabs/doku/pkg"
	"github.com/spf13/cobra"
)

func NewStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop Kubernetes cluster",
		Long:  `Stop Kubernetes cluster`,
		Run: func(cmd *cobra.Command, args []string) {
			spinner := pkg.NewSpinner()
			spinner.Start("Detecting the Kubernetes...")

			manager, err := pkg.GetClusterManager(spinner)
			if err != nil {
				spinner.Error("Failed get cluster information. Please run `doku init --overwrite` to reinitialise the project.", err)
			}

			if !manager.IsRunning() {
				spinner.Notice("No cluster is running.")
				spinner.StopSilently()
				return
			}

			spinner.UpdateMessage("Stopping %s cluster...", manager.Name())
			err = manager.Stop()
			if err != nil {
				spinner.Error("Failed stop %s cluster.", manager.Name(), err)
			}
			spinner.Stop("Stopped %s cluster.", manager.Name())
		},
	}

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// stopCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// stopCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
