/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Doku",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Doku CLI v0.1.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
