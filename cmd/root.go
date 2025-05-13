/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "doku",
	Short: "Doku is a unified CLI to Do Kubernetes",
	Long: `Doku (short for "Do Kubernetes") helps you interact with Kubernetes environments 
like minikube, k3s, and others through a common command-line interface.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to Doku! Run 'doku --help' to explore commands.")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
