package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/dokulabs/doku-cli/internal/service"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	execContainer   string
	execInteractive bool
	execTTY         bool
	execUser        string
	execWorkdir     string
)

var execCmd = &cobra.Command{
	Use:   "exec <service> [command...]",
	Short: "Execute a command in a running service container",
	Long: `Execute a command inside a running service container.

If no command is specified, starts an interactive shell (bash or sh).

Examples:
  doku exec postgres                    # Open shell in postgres container
  doku exec postgres psql -U postgres   # Run psql command
  doku exec redis redis-cli             # Run redis-cli
  doku exec myapp -- npm run migrate    # Run npm command (use -- for args)
  doku exec myapp -u root bash          # Run as root user`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE:               runExec,
}

func init() {
	rootCmd.AddCommand(execCmd)

	execCmd.Flags().StringVarP(&execContainer, "container", "c", "", "Container name (for multi-container services)")
	execCmd.Flags().BoolVarP(&execInteractive, "interactive", "i", true, "Keep STDIN open")
	execCmd.Flags().BoolVarP(&execTTY, "tty", "t", true, "Allocate a pseudo-TTY")
	execCmd.Flags().StringVarP(&execUser, "user", "u", "", "Username or UID")
	execCmd.Flags().StringVarP(&execWorkdir, "workdir", "w", "", "Working directory inside the container")
}

func runExec(cmd *cobra.Command, args []string) error {
	instanceName := args[0]

	// Determine the command to run
	var execCommand []string
	if len(args) > 1 {
		execCommand = args[1:]
	} else {
		// Default to shell
		execCommand = []string{"sh", "-c", "command -v bash > /dev/null && exec bash || exec sh"}
	}

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	if !cfgMgr.IsInitialized() {
		color.Yellow("Doku is not initialized. Run 'doku init' first.")
		return nil
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Handle Traefik specially
	var containerName string
	if instanceName == "traefik" || instanceName == "doku-traefik" {
		containerName = "doku-traefik"
	} else {
		// Get service instance
		serviceMgr := service.NewManager(dockerClient, cfgMgr)
		instance, err := serviceMgr.Get(instanceName)
		if err != nil {
			return fmt.Errorf("service '%s' not found", instanceName)
		}

		// Handle multi-container services
		if instance.IsMultiContainer {
			if execContainer == "" {
				// Show available containers
				fmt.Println()
				color.Yellow("'%s' is a multi-container service. Specify a container:", instance.Name)
				fmt.Println()
				for _, c := range instance.Containers {
					fmt.Printf("  • %s\n", c.Name)
				}
				fmt.Println()
				color.Cyan("Usage: doku exec %s --container <name> [command...]", instance.Name)
				fmt.Println()
				return nil
			}

			// Find the specified container
			found := false
			for _, c := range instance.Containers {
				if c.Name == execContainer {
					containerName = c.FullName
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("container '%s' not found in service '%s'", execContainer, instance.Name)
			}
		} else {
			containerName = instance.ContainerName
		}
	}

	// Check if container is running
	info, err := dockerClient.ContainerInspect(containerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	if !info.State.Running {
		return fmt.Errorf("container is not running. Start it first with: doku start %s", instanceName)
	}

	// Execute command
	ctx := context.Background()
	execOpts := docker.ExecOptions{
		Container:   containerName,
		Command:     execCommand,
		Interactive: execInteractive,
		TTY:         execTTY,
		User:        execUser,
		WorkDir:     execWorkdir,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}

	return dockerClient.Exec(ctx, execOpts)
}
