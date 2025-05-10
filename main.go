/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"doku/pkg/cluster"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cliName = "doku"
var cliDescription = "Doku is a CLI tool to manage local development using kubernetes"

var (
	logLevel    string
	version     = "dev"
	showVersion bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   cliName,
		Short: cliDescription,
		Long:  cliDescription,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if showVersion {
				fmt.Printf("%s version %s\n", cliName, version)
				os.Exit(0)
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Print the version of the CLI")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "Log level (debug, info, warn, error)")

	rootCmd.AddCommand(cluster.ManageCommand())

	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Printf("%s version: %s\n", cliName, version)
			os.Exit(0)
		}
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
