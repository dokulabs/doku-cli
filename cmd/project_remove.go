package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	projectRemoveImage bool
	projectRemoveYes   bool
)

// projectRemoveCmd represents the project remove command
var projectRemoveCmd = &cobra.Command{
	Use:     "remove [project-name]",
	Aliases: []string{"rm"},
	Short:   "Remove a project",
	Long: `Remove a project from Doku.

This command removes a project's configuration, stops and removes its container,
and optionally removes the built Docker image.

Examples:
  # Remove a project (interactive)
  doku project remove myapp

  # Remove project and image
  doku project remove myapp --image

  # Remove without confirmation
  doku project remove myapp --yes`,
	Args: cobra.ExactArgs(1),
	RunE: projectRemoveRun,
}

func init() {
	projectCmd.AddCommand(projectRemoveCmd)

	projectRemoveCmd.Flags().BoolVar(&projectRemoveImage, "image", false, "Also remove the Docker image")
	projectRemoveCmd.Flags().BoolVarP(&projectRemoveYes, "yes", "y", false, "Skip confirmation prompt")
}

func projectRemoveRun(cmd *cobra.Command, args []string) error {
	projectName := args[0]

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

	// Get project
	proj, err := projectMgr.Get(projectName)
	if err != nil {
		return fmt.Errorf("project '%s' not found", projectName)
	}

	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	// Show what will be removed
	fmt.Println()
	yellow.Printf("⚠️  Remove Project: %s\n", proj.Name)
	fmt.Println()
	fmt.Println("This will remove:")
	fmt.Printf("  • Container: %s\n", proj.ContainerName)
	fmt.Printf("  • Configuration for: %s\n", proj.Name)

	if projectRemoveImage {
		fmt.Printf("  • Docker image: doku-project-%s:latest\n", proj.Name)
	}

	// Confirm removal
	if !projectRemoveYes {
		fmt.Println()
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to remove '%s'?", projectName),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			fmt.Println()
			cyan.Println("→ Cancelled")
			fmt.Println()
			return nil
		}

		// Ask about image if not specified
		if !projectRemoveImage {
			fmt.Println()
			imagePrompt := &survey.Confirm{
				Message: "Do you want to remove the Docker image as well?",
				Default: false,
			}
			if err := survey.AskOne(imagePrompt, &projectRemoveImage); err != nil {
				return err
			}
		}
	}

	// Remove project
	fmt.Println()
	cyan.Printf("→ Removing project: %s\n", projectName)
	fmt.Println()

	if err := projectMgr.Remove(projectName, projectRemoveImage); err != nil {
		red := color.New(color.FgRed)
		red.Printf("✗ Failed to remove project: %v\n\n", err)
		return err
	}

	green := color.New(color.FgGreen)
	fmt.Println()
	green.Println("✓ Project removed successfully")
	fmt.Println()

	return nil
}
