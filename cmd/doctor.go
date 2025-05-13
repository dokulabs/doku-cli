/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/dokulabs/doku/pkg"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var installMissing bool

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check if your system has the necessary tools for Doku",
	Long:  `Check if your system has the necessary tools for Doku`,
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor(installMissing)
	},
}

func init() {
	doctorCmd.Flags().BoolVarP(&installMissing, "install", "i", false, "Attempt to install missing tools automatically")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(installMissing bool) {
	spinner := pkg.NewSpinner()
	spinner.Start("🔍 Running Doku system health check...")
	time.Sleep(2 * time.Second)

	osName := runtime.GOOS
	common := []string{"kubectl", "curl", "helm"}
	osTools := []string{"docker"}
	var missing []string

	// Define OS-specific tools
	switch osName {
	case "linux":
		if isWSL() {
			spinner.Success("📦 Detected WSL2 on Windows.")
		} else {
			spinner.Success("🐧 Detected Linux.")
		}

	case "darwin":
		spinner.Success("🍏 Detected macOS.")
		if !checkCommand("brew") {
			spinner.Error("Homebrew is not installed. Please install Homebrew manually.")
		}

	case "windows":
		if isWSL() {
			spinner.Success("📦 Detected WSL2 on Windows.")
		} else {
			spinner.Warning("⚠️  Doku requires WSL2 on Windows. Please install WSL2.")
			return
		}

	default:
		fmt.Printf("❌ Unsupported OS: %s\n", osName)
		return
	}

	// Combine common tools with OS-specific tools
	toolsToCheck := append(common, osTools...)
	spinner.UpdateMessage("Checking required tools...")
	// Check for required tools
	for _, tool := range toolsToCheck {
		if !checkCommand(tool) {
			missing = append(missing, tool)
		} else {
			spinner.Success(tool + " is already installed.")
		}
	}
	time.Sleep(1 * time.Second)
	if len(missing) > 0 {
		spinner.Info("❗ Missing tools:")
		for _, m := range missing {
			spinner.Notice(" - %s", m)
		}

		if installMissing {
			ensureTool(missing, osName, spinner)
		} else {
			spinner.Info("💡 Use '--install' or '-i' to install missing tools automatically.")
		}
	} else {
		spinner.Stop("All required tools are present!")
	}
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

func isUbuntu() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "ubuntu")
}

func runCommand(spinner *pkg.Spinner, cmd ...string) bool {
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err := c.Run()
	if err != nil {
		spinner.Warning("⚠️ Failed to run command: %s\n", strings.Join(cmd, " "))
		return false
	}
	return true
}

func checkCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func ensureTool(tools []string, osName string, spinner *pkg.Spinner) {
	var pkgManager string
	var installCmd []string

	switch osName {
	case "linux":
		if isWSL() || isUbuntu() {
			pkgManager = "apt"
			installCmd = []string{"sudo", "apt", "update"}
		}
	case "darwin":
		pkgManager = "brew"
		installCmd = []string{"brew", "update"}
	default:
		spinner.Error("🚫 Auto-install not supported on this OS.")
		return
	}

	spinner.Info("Installing missing tools using %s...\n", pkgManager)
	runCommand(spinner, installCmd...)

	for _, tool := range tools {
		var cmd []string
		switch pkgManager {
		case "apt":
			cmd = []string{"sudo", "apt", "install", "-y", tool}
		case "brew":
			cmd = []string{"brew", "install", tool}
		}

		spinner.UpdateMessage("Installing %s...", tool)
		installed := runCommand(spinner, cmd...)
		if installed {
			spinner.Success("Successfully installed %s", tool)
		}
	}
}
