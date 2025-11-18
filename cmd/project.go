package cmd

import (
	"github.com/spf13/cobra"
)

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage custom projects",
	Long: `Manage custom projects with Dockerfiles.

Projects are your custom applications that you want to run alongside
Doku catalog services. Doku will build your Docker images, manage
dependencies, and provide HTTPS access with clean URLs.

Examples:
  # Add a project
  doku project add ./my-app --name myapp --port 8080

  # Build a project
  doku project build myapp

  # Run a project
  doku project run myapp

  # List all projects
  doku project list

  # Remove a project
  doku project remove myapp`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
