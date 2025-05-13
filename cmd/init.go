/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/dokulabs/doku/pkg"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new project",
	Long:  `Initialize a new doku project.`,
	Run: func(cmd *cobra.Command, args []string) {
		spinner := pkg.NewSpinner()
		spinner.Start("Initializing doku project")
		overwrite, err := cmd.Flags().GetBool("overwrite")
		if err != nil {
			spinner.Error("Error reading --overwrite flag:", err)
			return
		}
		err = pkg.ConfigInit(overwrite, spinner)
		if err != nil {
			spinner.Error("Error initializing doku project:", err)
		}
		spinner.StopSilently()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool("overwrite", false, "Overwrite existing config")
}
