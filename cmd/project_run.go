package cmd

import (
	"fmt"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	projectRunBuild       bool
	projectRunInstallDeps bool
	projectRunDetach      bool
)

// projectRunCmd represents the project run command
var projectRunCmd = &cobra.Command{
	Use:   "run [project-name]",
	Short: "Run a project",
	Long: `Run a project's Docker container.

This command starts your project in a Docker container, connects it to
the Doku network, and makes it accessible via HTTPS (unless internal).

If dependencies are specified but not installed, you'll be prompted to
install them (or use --install-deps to install automatically).

Examples:
  # Run a project
  doku project run myapp

  # Build and run
  doku project run myapp --build

  # Auto-install missing dependencies
  doku project run myapp --install-deps

  # Run in foreground (see logs)
  doku project run myapp --detach=false`,
	Args: cobra.ExactArgs(1),
	RunE: projectRunRun,
}

func init() {
	projectCmd.AddCommand(projectRunCmd)

	projectRunCmd.Flags().BoolVar(&projectRunBuild, "build", false, "Build image before running")
	projectRunCmd.Flags().BoolVar(&projectRunInstallDeps, "install-deps", false, "Automatically install missing dependencies")
	projectRunCmd.Flags().BoolVarP(&projectRunDetach, "detach", "d", true, "Run in background")
}

func projectRunRun(cmd *cobra.Command, args []string) error {
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
		return fmt.Errorf("project '%s' not found. Add it first with: doku project add", projectName)
	}

	cyan := color.New(color.FgCyan)
	fmt.Println()
	cyan.Printf("→ Running project: %s\n", proj.Name)
	fmt.Println()

	// Check dependencies if not auto-installing
	if len(proj.Dependencies) > 0 && !projectRunInstallDeps {
		runner := project.NewRunner(dockerClient, cfgMgr)
		shouldInstall, err := runner.PromptInstallDependencies(proj)
		if err != nil {
			return err
		}

		if shouldInstall {
			if err := runner.InstallDependencies(proj); err != nil {
				return fmt.Errorf("failed to install dependencies: %w", err)
			}
		}
	}

	// Run project
	opts := project.RunOptions{
		Name:        projectName,
		Build:       projectRunBuild,
		InstallDeps: projectRunInstallDeps,
		Detach:      projectRunDetach,
	}

	if err := projectMgr.Run(opts); err != nil {
		red := color.New(color.FgRed)
		red.Printf("\n✗ Failed to run project: %v\n\n", err)
		return err
	}

	return nil
}
