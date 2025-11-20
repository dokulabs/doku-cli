package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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

	// Display projects in tabular format
	fmt.Println()

	// Create a new tabwriter
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Print header
	headerColor := color.New(color.Bold, color.FgCyan)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		headerColor.Sprint("NAME"),
		headerColor.Sprint("STATUS"),
		headerColor.Sprint("PORT"),
		headerColor.Sprint("DEPENDENCIES"),
		headerColor.Sprint("URL"),
	)

	// Print each project
	for _, proj := range projects {
		// Format name
		name := proj.Name

		// Format status (plain text to fix alignment)
		status := formatProjectStatusText(proj.Status)

		// Format port
		port := "-"
		if proj.Port > 0 {
			port = fmt.Sprintf("%d", proj.Port)
		}

		// Format dependencies
		deps := "-"
		if len(proj.Dependencies) > 0 {
			if len(proj.Dependencies) > 2 {
				deps = strings.Join(proj.Dependencies[:2], ", ") + "..."
			} else {
				deps = strings.Join(proj.Dependencies, ", ")
			}
		}

		// Format URL
		url := proj.URL
		if url == "" {
			url = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name,
			status,
			port,
			deps,
			url,
		)
	}

	w.Flush()
	fmt.Println()

	// Show summary
	color.Cyan("Total: %d project(s)", len(projects))
	fmt.Println()

	return nil
}

func formatProjectStatus(status types.ServiceStatus) string {
	switch status {
	case types.StatusRunning:
		return color.GreenString("Up")
	case types.StatusStopped:
		return color.YellowString("Exited")
	case types.StatusFailed:
		return color.RedString("Failed")
	default:
		return color.New(color.Faint).Sprint("Unknown")
	}
}

func formatProjectStatusText(status types.ServiceStatus) string {
	switch status {
	case types.StatusRunning:
		return "Up"
	case types.StatusStopped:
		return "Exited"
	case types.StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}
