/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cluster

import (
	"github.com/dokulabs/doku/pkg"
	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "A brief description of your command",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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

			spinner.UpdateMessage("Getting %s cluster status...", manager.Name())
			err = manager.Status()
			if err != nil {
				spinner.Error("Failed to get %s cluster status.", manager.Name(), err)
			}
			spinner.StopSilently()
		},
	}

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
