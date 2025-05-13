/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/dokulabs/doku/cmd/cluster"
	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage Kubernetes clusters (minikube, k3s, etc)",
	Long:  "Manage Kubernetes clusters (minikube, k3s, etc)",
}

func init() {
	clusterCmd.AddCommand(cluster.NewStartCmd())
	clusterCmd.AddCommand(cluster.NewStatusCmd())
	clusterCmd.AddCommand(cluster.NewStopCmd())
	rootCmd.AddCommand(clusterCmd)
}
