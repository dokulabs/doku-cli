package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	projectBuildNoCache bool
	projectBuildPull    bool
	projectBuildTag     string
)

// projectBuildCmd represents the project build command
var projectBuildCmd = &cobra.Command{
	Use:   "build [project-name]",
	Short: "Build a project's Docker image",
	Long: `Build a Docker image for a project using its Dockerfile.

The built image will be tagged as doku-project-{name}:latest by default.
You can specify a custom tag with the --tag flag.

Examples:
  # Build a project
  doku project build myapp

  # Build without cache
  doku project build myapp --no-cache

  # Build and pull latest base images
  doku project build myapp --pull

  # Build with custom tag
  doku project build myapp --tag myapp:v1.0.0`,
	Args: cobra.ExactArgs(1),
	RunE: projectBuildRun,
}

func init() {
	projectCmd.AddCommand(projectBuildCmd)

	projectBuildCmd.Flags().BoolVar(&projectBuildNoCache, "no-cache", false, "Build without using cache")
	projectBuildCmd.Flags().BoolVar(&projectBuildPull, "pull", false, "Pull base image before building")
	projectBuildCmd.Flags().StringVarP(&projectBuildTag, "tag", "t", "", "Custom tag for the image")
}

func projectBuildRun(cmd *cobra.Command, args []string) error {
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
	yellow := color.New(color.FgYellow)

	cyan.Printf("\n→ Building project: %s\n", proj.Name)
	cyan.Printf("  Path: %s\n", proj.Path)
	cyan.Printf("  Dockerfile: %s\n", proj.Dockerfile)

	// Load .env.doku from project directory
	envVars := make(map[string]string)
	envDokuPath := filepath.Join(proj.Path, ".env.doku")
	if project.FileExists(envDokuPath) {
		cyan.Printf("\n→ Loading environment variables from .env.doku\n")
		fileEnv, err := project.LoadEnvFile(envDokuPath)
		if err != nil {
			yellow.Printf("⚠️  Warning: Failed to load .env.doku: %v\n", err)
		} else {
			envVars = fileEnv
			fmt.Printf("  Loaded %d build arguments from .env.doku\n", len(fileEnv))
		}
	}

	// Build project with environment variables
	opts := project.BuildOptions{
		Name:      projectName,
		NoCache:   projectBuildNoCache,
		Pull:      projectBuildPull,
		Tag:       projectBuildTag,
		BuildArgs: envVars,
	}

	if err := projectMgr.Build(opts); err != nil {
		red := color.New(color.FgRed)
		red.Printf("\n✗ Build failed: %v\n\n", err)
		return err
	}

	// Show next steps
	fmt.Println()
	yellow.Println("Next step:")
	fmt.Printf("  Run: doku project run %s\n", projectName)
	fmt.Println()

	return nil
}
