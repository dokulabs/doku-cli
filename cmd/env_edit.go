package cmd

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/envfile"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var envEditCmd = &cobra.Command{
	Use:   "edit <service>",
	Short: "Edit environment variables for a service",
	Long: `Open the environment file for a service in your preferred editor.

The environment file is stored at:
  - Catalog services: ~/.doku/services/<service>.env
  - Custom projects:  ~/.doku/projects/<project>.env

The editor is selected from (in order):
  1. $EDITOR environment variable
  2. $VISUAL environment variable
  3. vim, vi, or nano (whichever is available)

After saving and closing the editor, you'll be prompted to recreate the
container to apply the changes.

Examples:
  doku env edit postgres     # Edit environment for postgres
  doku env edit myapp        # Edit environment for custom project
  EDITOR=nano doku env edit postgres  # Use nano as editor`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvEdit,
}

func init() {
	envCmd.AddCommand(envEditCmd)
}

func runEnvEdit(cmd *cobra.Command, args []string) error {
	serviceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Check if initialized
	if !cfgMgr.IsInitialized() {
		color.Yellow("⚠️  Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Create service manager
	serviceMgr := service.NewManager(dockerClient, cfgMgr)

	// Get instance
	instance, err := serviceMgr.Get(serviceName)
	if err != nil {
		return fmt.Errorf("service '%s' not found. Use 'doku list' to see installed services", serviceName)
	}

	isCustomProject := instance.ServiceType == "custom-project"

	// Get env file path
	envMgr := envfile.NewManager(cfgMgr.GetDokuDir())
	var envPath string
	if isCustomProject {
		envPath = envMgr.GetProjectEnvPath(serviceName)
	} else {
		envPath = envMgr.GetServiceEnvPath(serviceName, "")
	}

	// Ensure env file exists (migrate from config if needed)
	if !envMgr.Exists(envPath) {
		// Try to migrate from config
		var env map[string]string
		if isCustomProject {
			projectMgr, err := project.NewManager(dockerClient, cfgMgr)
			if err != nil {
				return fmt.Errorf("failed to create project manager: %w", err)
			}
			proj, err := projectMgr.Get(serviceName)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}
			env = proj.Environment
		} else {
			env = instance.Environment
		}

		if env == nil {
			env = make(map[string]string)
		}

		// Save to env file
		if err := envMgr.Save(envPath, env); err != nil {
			return fmt.Errorf("failed to create environment file: %w", err)
		}
		color.Green("✓ Created environment file: %s", envPath)
	}

	// Get file info before editing
	infoBefore, err := os.Stat(envPath)
	if err != nil {
		return fmt.Errorf("failed to stat env file: %w", err)
	}

	// Show file location
	fmt.Println()
	color.Cyan("Opening environment file in editor...")
	fmt.Printf("File: %s\n", envPath)
	fmt.Println()

	// Open in editor
	if err := envfile.OpenInEditor(envPath); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	// Check if file was modified
	infoAfter, err := os.Stat(envPath)
	if err != nil {
		return fmt.Errorf("failed to stat env file after editing: %w", err)
	}

	if infoAfter.ModTime() == infoBefore.ModTime() {
		fmt.Println()
		color.Yellow("No changes made")
		return nil
	}

	// Validate the env file
	_, err = envMgr.Load(envPath)
	if err != nil {
		return fmt.Errorf("invalid environment file format: %w", err)
	}

	color.Green("✓ Environment file saved")
	fmt.Println()

	// Ask if user wants to recreate the service to apply changes
	color.Yellow("⚠️  Environment variables require container recreation to take effect")
	fmt.Println()
	recreate := false
	prompt := &survey.Confirm{
		Message: "Recreate the container to apply changes?",
		Default: true,
	}
	if err := survey.AskOne(prompt, &recreate); err != nil {
		return err
	}

	if recreate {
		fmt.Println()
		color.Cyan("Recreating container to apply environment changes...")
		fmt.Println()

		if isCustomProject {
			// For custom projects, use project manager
			projectMgr, err := project.NewManager(dockerClient, cfgMgr)
			if err != nil {
				return fmt.Errorf("failed to create project manager: %w", err)
			}

			runOpts := project.RunOptions{
				Name:   serviceName,
				Build:  false, // Don't rebuild image
				Detach: true,
			}
			if err := projectMgr.Run(runOpts); err != nil {
				return fmt.Errorf("failed to recreate container: %w", err)
			}
		} else {
			// For catalog services, use service manager
			if err := serviceMgr.Recreate(serviceName); err != nil {
				return fmt.Errorf("failed to recreate container: %w", err)
			}
		}

		fmt.Println()
		color.Green("✓ Container recreated successfully with new environment variables")
		fmt.Println()
	} else {
		fmt.Println()
		color.Yellow("⚠️  Changes saved but not applied.")
		color.Yellow("    To apply changes, restart the service:")
		color.Yellow("    doku restart %s", serviceName)
		fmt.Println()
	}

	return nil
}
