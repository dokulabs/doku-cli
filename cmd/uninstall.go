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
	Short: "Uninstall Doku and clean up everything",
	Long: `Uninstall Doku and clean up all resources including:
  • Docker containers (Traefik and all services)
  • Docker volumes
  • Docker network
  • Configuration directory (~/.doku/)
  • SSL certificates (with instructions)
  • DNS entries (with instructions)
  • Doku binary (with OS-specific commands)`,
	RunE: runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().BoolVarP(&uninstallForce, "force", "f", false, "Skip confirmation prompts")
	uninstallCmd.Flags().BoolVarP(&uninstallAll, "all", "a", false, "Remove everything including mkcert CA certificates")
}

func runUninstall(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Colors
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("\n%s\n\n", red("⚠️  Doku Uninstall"))
	fmt.Println("This will remove:")
	fmt.Printf("  • All Docker containers managed by Doku\n")
	fmt.Printf("  • All Docker volumes created by Doku\n")
	fmt.Printf("  • Doku Docker network\n")
	fmt.Printf("  • Configuration directory (~/.doku/)\n")

	if uninstallAll {
		fmt.Printf("  • mkcert CA certificates (--all flag)\n")
	}

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

	// Track what was cleaned up
	var cleaned []string
	var remaining []string

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

	// Step 2: Remove Docker volumes
	fmt.Printf("\n%s Removing Docker volumes...\n", cyan("→"))
	if dockerClient != nil {
		volumes, err := dockerClient.ListVolumes(ctx)
		if err != nil {
			fmt.Printf("  %s Failed to list volumes: %v\n", red("✗"), err)
		} else {
			volumesRemoved := 0
			for _, volume := range volumes {
				// Only process volumes with "doku-" prefix
				if !strings.HasPrefix(volume.Name, "doku-") {
					continue
				}

				if err := dockerClient.RemoveVolume(ctx, volume.Name); err != nil {
					fmt.Printf("  %s Failed to remove volume %s: %v\n", red("✗"), volume.Name, err)
				} else {
					fmt.Printf("  %s Removed volume %s\n", green("✓"), volume.Name)
					volumesRemoved++
				}
			}
			if volumesRemoved > 0 {
				cleaned = append(cleaned, fmt.Sprintf("%d Docker volume(s)", volumesRemoved))
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

	// Step 4: Remove config directory
	fmt.Printf("\n%s Removing configuration directory...\n", cyan("→"))
	if cfgMgr != nil {
		dokuDir := cfgMgr.GetDokuDir()
		if _, err := os.Stat(dokuDir); err == nil {
			if err := os.RemoveAll(dokuDir); err != nil {
				fmt.Printf("  %s Failed to remove %s: %v\n", red("✗"), dokuDir, err)
				remaining = append(remaining, fmt.Sprintf("Config directory: %s", dokuDir))
			} else {
				fmt.Printf("  %s Removed %s\n", green("✓"), dokuDir)
				cleaned = append(cleaned, "Configuration directory")
			}
		} else {
			fmt.Printf("  %s Config directory not found (already clean)\n", green("✓"))
		}
	}

	// Step 5: Uninstall mkcert CA (if --all flag)
	if uninstallAll {
		fmt.Printf("\n%s Uninstalling mkcert CA certificates...\n", cyan("→"))
		// This is optional and requires manual intervention
		remaining = append(remaining, "mkcert CA certificates - Run: mkcert -uninstall")
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
	currentExe, _ := os.Executable()
	currentExe, _ = filepath.EvalSymlinks(currentExe)

	selfDeleteFailed := false
	for _, binPath := range binaryPaths {
		if _, err := os.Stat(binPath); err == nil {
			// Resolve symlinks for comparison
			resolvedPath, _ := filepath.EvalSymlinks(binPath)

			// File exists, try to remove it
			if err := os.Remove(binPath); err != nil {
				// Check if this is the currently running binary
				if resolvedPath == currentExe {
					// Can't delete currently running binary on Unix systems
					selfDeleteFailed = true
					remaining = append(remaining, fmt.Sprintf("Binary: %s (currently running - will be removed after this command exits)", binPath))
				} else if os.IsPermission(err) {
					// If removal fails due to permissions, suggest sudo
					remaining = append(remaining, fmt.Sprintf("Binary: %s (requires elevated permissions: sudo rm %s)", binPath, binPath))
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

	// Store paths for deferred removal
	var pathsToRemoveAfterExit []string
	if selfDeleteFailed {
		for _, binPath := range binaryPaths {
			if _, err := os.Stat(binPath); err == nil {
				pathsToRemoveAfterExit = append(pathsToRemoveAfterExit, binPath)
			}
		}
	}

	// Print summary
	fmt.Printf("\n%s\n\n", green("✓ Cleanup Complete"))

	if len(cleaned) > 0 {
		fmt.Println(green("Removed:"))
		for _, item := range cleaned {
			fmt.Printf("  • %s\n", item)
		}
		fmt.Println()
	}

	// Print remaining items with OS-specific instructions
	if len(remaining) > 0 {
		fmt.Println(yellow("Additional Manual Steps Required:"))
		fmt.Println()
		for _, item := range remaining {
			fmt.Printf("  • %s\n", item)
		}
		fmt.Println()
	}

	// DNS entries
	fmt.Println(yellow("Manual Cleanup Recommendations:"))
	fmt.Println()

	nextNum := 1
	fmt.Printf("%s DNS Entries (in /etc/hosts or resolver)\n", yellow(fmt.Sprintf("%d.", nextNum)))
	switch runtime.GOOS {
	case "darwin": // macOS
		fmt.Println("   Remove entries from /etc/hosts:")
		fmt.Printf("   %s\n", cyan("sudo sed -i '' '/doku.local/d' /etc/hosts"))
		fmt.Println()
		fmt.Println("   If using resolver:")
		fmt.Printf("   %s\n", cyan("sudo rm -f /etc/resolver/doku.local"))
	case "linux":
		fmt.Println("   Remove entries from /etc/hosts:")
		fmt.Printf("   %s\n", cyan("sudo sed -i '/doku.local/d' /etc/hosts"))
	case "windows":
		fmt.Println("   Remove entries from hosts file:")
		fmt.Printf("   %s\n", cyan(`notepad C:\Windows\System32\drivers\etc\hosts`))
		fmt.Println("   Then manually remove lines containing 'doku.local'")
	}
	fmt.Println()

	// mkcert certificates
	if !uninstallAll {
		nextNum++
		fmt.Printf("%s mkcert CA Certificates (optional)\n", yellow(fmt.Sprintf("%d.", nextNum)))
		fmt.Println("   To remove the local CA certificates:")
		fmt.Printf("   %s\n", cyan("mkcert -uninstall"))
		fmt.Println("   Note: This will affect other apps using mkcert")
		fmt.Println()
	}

	// If we couldn't delete the currently running binary, provide a command to do it
	if len(pathsToRemoveAfterExit) > 0 {
		fmt.Println(yellow("\nTo complete the uninstall, run this command after Doku exits:"))
		switch runtime.GOOS {
		case "darwin", "linux":
			cmdParts := []string{"rm", "-f"}
			cmdParts = append(cmdParts, pathsToRemoveAfterExit...)
			fmt.Printf("   %s\n", cyan(strings.Join(cmdParts, " ")))
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
