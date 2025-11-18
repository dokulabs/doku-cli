package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/project"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	projectAddName       string
	projectAddDockerfile string
	projectAddPort       string
	projectAddPorts      []string
	projectAddEnv        []string
	projectAddDepends    []string
	projectAddDomain     string
	projectAddInternal   bool
)

// projectAddCmd represents the project add command
var projectAddCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a new project",
	Long: `Add a new project to Doku.

This command registers a project with Doku, making it available for building
and running. The project directory must contain a Dockerfile.

Examples:
  # Add project from current directory
  doku project add .

  # Add with custom name and port
  doku project add ./my-app --name myapp --port 8080

  # Add with dependencies
  doku project add ./backend \
    --name api \
    --port 8080 \
    --depends postgres:16,redis

  # Add with environment variables
  doku project add ./app \
    --name myapp \
    --env NODE_ENV=development \
    --env API_KEY=secret

  # Add as internal service (no HTTPS)
  doku project add ./worker --internal`,
	Args: cobra.ExactArgs(1),
	RunE: projectAddRun,
}

func init() {
	projectCmd.AddCommand(projectAddCmd)

	projectAddCmd.Flags().StringVarP(&projectAddName, "name", "n", "", "Project name (defaults to directory name)")
	projectAddCmd.Flags().StringVar(&projectAddDockerfile, "dockerfile", "", "Path to Dockerfile (default: ./Dockerfile)")
	projectAddCmd.Flags().StringVarP(&projectAddPort, "port", "p", "", "Main port to expose")
	projectAddCmd.Flags().StringSliceVar(&projectAddPorts, "ports", []string{}, "Additional port mappings (host:container)")
	projectAddCmd.Flags().StringSliceVarP(&projectAddEnv, "env", "e", []string{}, "Environment variables (KEY=VALUE)")
	projectAddCmd.Flags().StringSliceVar(&projectAddDepends, "depends", []string{}, "Service dependencies (e.g., postgres:16,redis)")
	projectAddCmd.Flags().StringVar(&projectAddDomain, "domain", "", "Custom domain (default: doku.local)")
	projectAddCmd.Flags().BoolVar(&projectAddInternal, "internal", false, "Internal only (no Traefik/HTTPS)")
}

func projectAddRun(cmd *cobra.Command, args []string) error {
	projectPath := args[0]

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

	// Parse port
	var mainPort int
	if projectAddPort != "" {
		mainPort, err = strconv.Atoi(projectAddPort)
		if err != nil {
			return fmt.Errorf("invalid port number: %s", projectAddPort)
		}
	}

	// Parse environment variables
	envMap := make(map[string]string)
	for _, envVar := range projectAddEnv {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid environment variable format: %s (use KEY=VALUE)", envVar)
		}
		envMap[parts[0]] = parts[1]
	}

	// Parse dependencies
	var dependencies []string
	if len(projectAddDepends) > 0 {
		// If only one item, might be comma-separated
		if len(projectAddDepends) == 1 && strings.Contains(projectAddDepends[0], ",") {
			dependencies = strings.Split(projectAddDepends[0], ",")
		} else {
			dependencies = projectAddDepends
		}

		// Trim whitespace
		for i := range dependencies {
			dependencies[i] = strings.TrimSpace(dependencies[i])
		}
	}

	// Add project
	opts := project.AddOptions{
		ProjectPath:  projectPath,
		Name:         projectAddName,
		Dockerfile:   projectAddDockerfile,
		Port:         mainPort,
		Ports:        projectAddPorts,
		Environment:  envMap,
		Dependencies: dependencies,
		Domain:       projectAddDomain,
		Internal:     projectAddInternal,
	}

	proj, err := projectMgr.Add(opts)
	if err != nil {
		return err
	}

	// Display success message
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)

	fmt.Println()
	green.Println("✓ Project added successfully")
	fmt.Println()

	// Show project details
	fmt.Println("Project Details:")
	cyan.Printf("  Name: %s\n", proj.Name)
	cyan.Printf("  Path: %s\n", proj.Path)
	cyan.Printf("  Dockerfile: %s\n", proj.Dockerfile)

	if proj.Port > 0 {
		cyan.Printf("  Port: %d\n", proj.Port)
	}

	if proj.URL != "" {
		cyan.Printf("  URL: %s\n", proj.URL)
	}

	if len(proj.Dependencies) > 0 {
		cyan.Printf("  Dependencies: %s\n", strings.Join(proj.Dependencies, ", "))
	}

	// Show next steps
	fmt.Println()
	yellow.Println("Next steps:")
	fmt.Printf("  1. Build: doku project build %s\n", proj.Name)
	fmt.Printf("  2. Run:   doku project run %s\n", proj.Name)
	fmt.Println()

	return nil
}
