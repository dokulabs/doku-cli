package cmd

import (
	"fmt"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/pkg/types"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// projectListCmd represents the project list command
var projectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all projects",
	Long: `List all registered projects with their status.

This command displays a table of all projects that have been added to Doku,
including their current status, URL, and dependencies.

Examples:
  # List all projects
  doku project list

  # Short form
  doku project ls`,
	RunE: projectListRun,
}

func init() {
	projectCmd.AddCommand(projectListCmd)
}

func projectListRun(cmd *cobra.Command, args []string) error {
	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Initialize config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	// Initialize project manager
	projectMgr, err := project.NewManager(dockerClient, cfgMgr)
	if err != nil {
		return fmt.Errorf("failed to initialize project manager: %w", err)
	}

	// Get all projects
	projects, err := projectMgr.List()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		yellow := color.New(color.FgYellow)
		fmt.Println()
		yellow.Println("No projects found")
		fmt.Println()
		fmt.Println("Add a project with:")
		fmt.Println("  doku project add <path>")
		fmt.Println()
		return nil
	}

	// Display projects
	fmt.Println()
	fmt.Println("PROJECTS:")
	fmt.Println()

	// Color helpers
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	gray := color.New(color.FgHiBlack)
	cyan := color.New(color.FgCyan)

	for _, proj := range projects {
		// Format status with color
		var statusStr string
		switch proj.Status {
		case types.StatusRunning:
			statusStr = green.Sprint("● running")
		case types.StatusStopped:
			statusStr = gray.Sprint("○ stopped")
		case types.StatusFailed:
			statusStr = red.Sprint("✗ failed")
		default:
			statusStr = yellow.Sprint("? unknown")
		}

		// Print project info
		fmt.Printf("  %s %-20s %s\n", statusStr, proj.Name, cyan.Sprint(proj.URL))

		// Show additional info
		if proj.Port > 0 {
			fmt.Printf("    Port: %d\n", proj.Port)
		}
		if len(proj.Dependencies) > 0 {
			fmt.Printf("    Dependencies: %s\n", strings.Join(proj.Dependencies, ", "))
		}
		fmt.Println()
	}

	// Show summary
	cyan.Printf("Total: %d project(s)\n", len(projects))
	fmt.Println()

	return nil
}
