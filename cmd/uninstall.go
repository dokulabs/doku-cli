package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	uninstallForce bool
	uninstallAll   bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Doku and clean up containers",
	Long: `Uninstall Doku and clean up containers and configuration.

This will remove:
  • Docker containers (Traefik and all services)
  • Docker network
  • Configuration file (~/.doku/config.toml)
  • SSL certificates

Data is preserved for safety:
  • Docker volumes (your data) are NOT removed
  • Environment files (~/.doku/services/*.env) are NOT removed

After uninstall, manual cleanup instructions will be shown if you want to
permanently delete the data.

Use --force to skip confirmation prompts.
Use --all to also show instructions for removing mkcert CA certificates.`,
	RunE: runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVarP(&uninstallForce, "force", "f", false, "Skip confirmation prompts")
	uninstallCmd.Flags().BoolVarP(&uninstallAll, "all", "a", false, "Show instructions for removing mkcert CA certificates")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Colors
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("\n%s\n\n", yellow("⚠️  Doku Uninstall"))
	fmt.Println("This will remove:")
	fmt.Printf("  • All Docker containers managed by Doku\n")
	fmt.Printf("  • Doku Docker network\n")
	fmt.Printf("  • Configuration file (~/.doku/config.toml)\n")
	fmt.Printf("  • SSL certificates\n")
	fmt.Println()
	fmt.Println(green("Data preserved for safety:"))
	fmt.Printf("  • Docker volumes (your data)\n")
	fmt.Printf("  • Environment files (~/.doku/services/*.env)\n")
	fmt.Println()

	// Confirmation
	if !uninstallForce {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Are you sure you want to uninstall Doku?",
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if !confirm {
			fmt.Println(yellow("Uninstall cancelled"))
			return nil
		}
	}

	fmt.Println()

	// Track what was cleaned up and what data is preserved
	var cleaned []string
	var remaining []string
	var preservedVolumes []string
	var preservedEnvFiles []string

	// Initialize config manager
	cfgMgr, err := config.New()
	if err != nil {
		fmt.Printf("%s Warning: Could not initialize config manager: %v\n", yellow("⚠"), err)
	}

	// Initialize Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		fmt.Printf("%s Warning: Could not connect to Docker: %v\n", yellow("⚠"), err)
		fmt.Println("  Some cleanup steps will be skipped")
	}

	// Step 1: Stop and remove all containers
	fmt.Printf("%s Stopping and removing Docker containers...\n", cyan("→"))
	if dockerClient != nil {
		containersRemoved := 0

		// Get all containers with name starting with "doku-"
		allContainers, err := dockerClient.ListContainers(ctx)
		if err != nil {
			fmt.Printf("  %s Failed to list containers: %v\n", red("✗"), err)
		} else {
			for _, container := range allContainers {
				name := strings.TrimPrefix(container.Names[0], "/")

				// Only process containers with "doku-" prefix
				if !strings.HasPrefix(name, "doku-") {
					continue
				}

				if err := dockerClient.StopContainer(ctx, container.ID); err != nil {
					fmt.Printf("  %s Failed to stop %s: %v\n", red("✗"), name, err)
				} else {
					fmt.Printf("  %s Stopped %s\n", green("✓"), name)
				}

				if err := dockerClient.RemoveContainer(ctx, container.ID); err != nil {
					fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), name, err)
				} else {
					fmt.Printf("  %s Removed %s\n", green("✓"), name)
					containersRemoved++
				}
			}
		}

		if containersRemoved > 0 {
			cleaned = append(cleaned, fmt.Sprintf("%d Docker container(s)", containersRemoved))
		}
	}

	// Step 2: List Docker volumes (but don't remove them)
	fmt.Printf("\n%s Checking Docker volumes (preserving data)...\n", cyan("→"))
	if dockerClient != nil {
		volumes, err := dockerClient.ListVolumes(ctx)
		if err != nil {
			fmt.Printf("  %s Failed to list volumes: %v\n", red("✗"), err)
		} else {
			for _, volume := range volumes {
				// Only count volumes with "doku-" prefix
				if strings.HasPrefix(volume.Name, "doku-") {
					preservedVolumes = append(preservedVolumes, volume.Name)
				}
			}
			if len(preservedVolumes) > 0 {
				fmt.Printf("  %s Preserved %d Docker volume(s) with your data\n", green("✓"), len(preservedVolumes))
			}
		}
	}

	// Step 3: Remove Docker network
	fmt.Printf("\n%s Removing Docker network...\n", cyan("→"))
	if dockerClient != nil {
		networkName := "doku-network"
		if err := dockerClient.RemoveNetwork(ctx, networkName); err != nil {
			if !strings.Contains(err.Error(), "not found") {
				fmt.Printf("  %s Failed to remove network: %v\n", red("✗"), err)
			}
		} else {
			fmt.Printf("  %s Removed network %s\n", green("✓"), networkName)
			cleaned = append(cleaned, "Docker network")
		}
	}

	// Step 4: List env files (but don't remove them)
	fmt.Printf("\n%s Checking environment files (preserving data)...\n", cyan("→"))
	if cfgMgr != nil {
		dokuDir := cfgMgr.GetDokuDir()
		servicesDir := filepath.Join(dokuDir, "services")
		projectsDir := filepath.Join(dokuDir, "projects")

		// Check services env files
		if entries, err := os.ReadDir(servicesDir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".env") {
					preservedEnvFiles = append(preservedEnvFiles, filepath.Join(servicesDir, entry.Name()))
				}
			}
		}

		// Check projects env files
		if entries, err := os.ReadDir(projectsDir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".env") {
					preservedEnvFiles = append(preservedEnvFiles, filepath.Join(projectsDir, entry.Name()))
				}
			}
		}

		if len(preservedEnvFiles) > 0 {
			fmt.Printf("  %s Preserved %d environment file(s)\n", green("✓"), len(preservedEnvFiles))
		}
	}

	// Step 5: Remove config file and certs (but keep env files)
	fmt.Printf("\n%s Removing configuration and certificates...\n", cyan("→"))
	if cfgMgr != nil {
		dokuDir := cfgMgr.GetDokuDir()

		// Remove config.toml
		configPath := filepath.Join(dokuDir, "config.toml")
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), configPath, err)
			} else {
				fmt.Printf("  %s Removed %s\n", green("✓"), configPath)
				cleaned = append(cleaned, "Configuration file")
			}
		}

		// Remove certs directory
		certsDir := filepath.Join(dokuDir, "certs")
		if _, err := os.Stat(certsDir); err == nil {
			if err := os.RemoveAll(certsDir); err != nil {
				fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), certsDir, err)
			} else {
				fmt.Printf("  %s Removed %s\n", green("✓"), certsDir)
				cleaned = append(cleaned, "SSL certificates")
			}
		}

		// Remove traefik directory
		traefikDir := filepath.Join(dokuDir, "traefik")
		if _, err := os.Stat(traefikDir); err == nil {
			if err := os.RemoveAll(traefikDir); err != nil {
				fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), traefikDir, err)
			} else {
				fmt.Printf("  %s Removed %s\n", green("✓"), traefikDir)
				cleaned = append(cleaned, "Traefik configuration")
			}
		}

		// Remove catalog directory
		catalogDir := filepath.Join(dokuDir, "catalog")
		if _, err := os.Stat(catalogDir); err == nil {
			if err := os.RemoveAll(catalogDir); err != nil {
				fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), catalogDir, err)
			} else {
				fmt.Printf("  %s Removed %s\n", green("✓"), catalogDir)
				cleaned = append(cleaned, "Catalog cache")
			}
		}
	}

	// Step 6: Remove Doku binaries
	fmt.Printf("\n%s Removing Doku binaries...\n", cyan("→"))
	homeDir, _ := os.UserHomeDir()

	binariesRemoved := 0
	binaryPaths := []string{
		filepath.Join(homeDir, "go", "bin", "doku"),
		filepath.Join(homeDir, "go", "bin", "doku-cli"),
		"/usr/local/bin/doku",
		"/usr/local/bin/doku-cli",
	}

	if runtime.GOOS == "windows" {
		binaryPaths = []string{
			filepath.Join(homeDir, "go", "bin", "doku.exe"),
			filepath.Join(homeDir, "go", "bin", "doku-cli.exe"),
		}
	}

	// Get the current executable path
	currentExe, err := os.Executable()
	if err != nil {
		currentExe = ""
	} else {
		if resolved, err := filepath.EvalSymlinks(currentExe); err == nil {
			currentExe = resolved
		}
	}

	selfDeleteFailed := false
	var pathsToRemoveAfterExit []string
	for _, binPath := range binaryPaths {
		if _, err := os.Stat(binPath); err == nil {
			resolvedPath := binPath
			if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
				resolvedPath = resolved
			}

			if err := os.Remove(binPath); err != nil {
				if resolvedPath == currentExe {
					selfDeleteFailed = true
					pathsToRemoveAfterExit = append(pathsToRemoveAfterExit, binPath)
					remaining = append(remaining, fmt.Sprintf("Binary: %s (currently running)", binPath))
				} else if os.IsPermission(err) {
					remaining = append(remaining, fmt.Sprintf("Binary: %s (requires sudo)", binPath))
				} else {
					fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), binPath, err)
					remaining = append(remaining, fmt.Sprintf("Binary: %s", binPath))
				}
			} else {
				fmt.Printf("  %s Removed %s\n", green("✓"), binPath)
				binariesRemoved++
			}
		}
	}

	if binariesRemoved > 0 {
		cleaned = append(cleaned, fmt.Sprintf("%d binary/binaries", binariesRemoved))
	}

	// Print summary
	fmt.Printf("\n%s\n\n", green("✓ Uninstall Complete"))

	if len(cleaned) > 0 {
		fmt.Println(green("Removed:"))
		for _, item := range cleaned {
			fmt.Printf("  • %s\n", item)
		}
		fmt.Println()
	}

	// Show preserved data
	if len(preservedVolumes) > 0 || len(preservedEnvFiles) > 0 {
		fmt.Println(green("Data preserved for safety:"))
		if len(preservedVolumes) > 0 {
			fmt.Printf("  • %d Docker volume(s)\n", len(preservedVolumes))
		}
		if len(preservedEnvFiles) > 0 {
			fmt.Printf("  • %d environment file(s)\n", len(preservedEnvFiles))
		}
		fmt.Println()
	}

	// Show cleanup instructions for preserved data
	if len(preservedVolumes) > 0 || len(preservedEnvFiles) > 0 {
		fmt.Println(yellow("To permanently delete your data (cannot be undone):"))
		fmt.Println()

		if len(preservedVolumes) > 0 {
			fmt.Println(color.New(color.Bold).Sprint("Docker volumes:"))
			fmt.Printf("  %s\n", cyan("# Remove all doku volumes"))
			fmt.Printf("  %s\n", cyan("docker volume ls -q | grep doku- | xargs docker volume rm"))
			fmt.Println()
			fmt.Println("  Or remove individually:")
			for _, vol := range preservedVolumes {
				fmt.Printf("  docker volume rm %s\n", vol)
			}
			fmt.Println()
		}

		if len(preservedEnvFiles) > 0 {
			fmt.Println(color.New(color.Bold).Sprint("Environment files:"))
			dokuDir := cfgMgr.GetDokuDir()
			fmt.Printf("  %s\n", cyan(fmt.Sprintf("rm -rf %s/services/*.env %s/projects/*.env", dokuDir, dokuDir)))
			fmt.Println()
		}
	}

	// Print remaining items
	if len(remaining) > 0 {
		fmt.Println(yellow("Manual steps required:"))
		for _, item := range remaining {
			fmt.Printf("  • %s\n", item)
		}
		fmt.Println()
	}

	// DNS entries
	fmt.Println(yellow("Additional cleanup (optional):"))
	fmt.Println()

	fmt.Printf("%s DNS Entries (in /etc/hosts)\n", yellow("1."))
	switch runtime.GOOS {
	case "darwin":
		fmt.Printf("   %s\n", cyan("sudo sed -i '' '/doku.local/d' /etc/hosts"))
		fmt.Println("   If using resolver:")
		fmt.Printf("   %s\n", cyan("sudo rm -f /etc/resolver/doku.local"))
	case "linux":
		fmt.Printf("   %s\n", cyan("sudo sed -i '/doku.local/d' /etc/hosts"))
	case "windows":
		fmt.Printf("   %s\n", cyan(`notepad C:\Windows\System32\drivers\etc\hosts`))
		fmt.Println("   Then manually remove lines containing 'doku.local'")
	}
	fmt.Println()

	// mkcert certificates
	if uninstallAll {
		fmt.Printf("%s mkcert CA Certificates\n", yellow("2."))
		fmt.Printf("   %s\n", cyan("mkcert -uninstall"))
		fmt.Println("   Note: This will affect other apps using mkcert")
		fmt.Println()
	}

	// If we couldn't delete the currently running binary
	if selfDeleteFailed && len(pathsToRemoveAfterExit) > 0 {
		fmt.Println(yellow("To remove the doku binary after this command exits:"))
		switch runtime.GOOS {
		case "darwin", "linux":
			fmt.Printf("   %s\n", cyan(fmt.Sprintf("rm -f %s", strings.Join(pathsToRemoveAfterExit, " "))))
		case "windows":
			for _, path := range pathsToRemoveAfterExit {
				fmt.Printf("   %s\n", cyan(fmt.Sprintf("del %s", path)))
			}
		}
		fmt.Println()
	}

	fmt.Println(green("Thank you for using Doku! 👋"))
	fmt.Println()

	return nil
}
