package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

// K3sManager implements ClusterManager interface
type K3sManager struct{}

// spinnerConfig defines the spinner settings
type spinnerConfig struct {
	message string
	delay   time.Duration
}

// withSpinner runs a function with a spinner animation
func withSpinner(ctx context.Context, config spinnerConfig, fn func() error) error {
	s := spinner.New(spinner.CharSets[14], config.delay)
	s.Suffix = fmt.Sprintf(" %s...", config.message)
	s.Start()
	defer s.Stop()

	err := fn()
	if err != nil {
		s.FinalMSG = fmt.Sprintf("❌ %s failed\n", config.message)
		return err
	}
	s.FinalMSG = fmt.Sprintf("✅ %s completed\n", config.message)
	return nil
}

// IsInstalled checks if k3s is installed
func (k *K3sManager) IsInstalled() bool {
	_, err := exec.LookPath("k3s")
	return err == nil
}

// Install handles k3s installation across supported platforms
func (k *K3sManager) Install() error {
	ctx := context.Background()
	spinnerCfg := spinnerConfig{
		message: "Installing k3s",
		delay:   100 * time.Millisecond,
	}

	return withSpinner(ctx, spinnerCfg, func() error {
		switch runtime.GOOS {
		case "linux":
			cmd := exec.Command("sh", "-c", "curl -sfL https://get.k3s.io | sh -")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "darwin":
			// Check if Homebrew is installed
			if _, err := exec.LookPath("brew"); err != nil {
				return fmt.Errorf("Homebrew is required for macOS installation. Please install it from https://brew.sh")
			}
			// Install k3s using Homebrew
			cmd := exec.Command("brew", "install", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "windows":
			// Check if WSL2 is available
			if _, err := exec.LookPath("wsl"); err != nil {
				return fmt.Errorf("WSL2 is required for Windows installation. Please enable it following https://learn.microsoft.com/en-us/windows/wsl/install")
			}
			// Install k3s in WSL2 Ubuntu
			cmd := exec.Command("wsl", "-d", "Ubuntu", "--", "sh", "-c", "curl -sfL https://get.k3s.io | sh -")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		default:
			return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		}
	})
}

// Uninstall removes k3s from the system
func (k *K3sManager) Uninstall() error {
	ctx := context.Background()
	spinnerCfg := spinnerConfig{
		message: "Uninstalling k3s",
		delay:   100 * time.Millisecond,
	}

	if !k.IsInstalled() {
		return fmt.Errorf("k3s is not installed")
	}

	return withSpinner(ctx, spinnerCfg, func() error {
		switch runtime.GOOS {
		case "linux":
			cmd := exec.Command("sh", "-c", "/usr/local/bin/k3s-uninstall.sh")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "darwin":
			cmd := exec.Command("brew", "uninstall", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "windows":
			cmd := exec.Command("wsl", "-d", "Ubuntu", "--", "sh", "-c", "/usr/local/bin/k3s-uninstall.sh")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		default:
			return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		}
	})
}

// IsRunning checks if k3s is currently running
func (k *K3sManager) IsRunning() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("pgrep", "k3s")
		return cmd.Run() == nil
	case "windows":
		cmd := exec.Command("wsl", "-d", "Ubuntu", "--", "pgrep", "k3s")
		return cmd.Run() == nil
	default:
		return false
	}
}

// Start launches the k3s service
func (k *K3sManager) Start() error {
	ctx := context.Background()
	spinnerCfg := spinnerConfig{
		message: "Starting k3s",
		delay:   100 * time.Millisecond,
	}

	if k.IsRunning() {
		return fmt.Errorf("k3s is already running")
	}

	return withSpinner(ctx, spinnerCfg, func() error {
		switch runtime.GOOS {
		case "linux":
			cmd := exec.Command("sudo", "systemctl", "start", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "darwin":
			cmd := exec.Command("brew", "services", "start", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "windows":
			cmd := exec.Command("wsl", "-d", "Ubuntu", "--", "sudo", "systemctl", "start", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		default:
			return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		}
	})
}

// Stop terminates the k3s service
func (k *K3sManager) Stop() error {
	ctx := context.Background()
	spinnerCfg := spinnerConfig{
		message: "Stopping k3s",
		delay:   100 * time.Millisecond,
	}

	if !k.IsRunning() {
		return fmt.Errorf("k3s is not running")
	}

	return withSpinner(ctx, spinnerCfg, func() error {
		switch runtime.GOOS {
		case "linux":
			cmd := exec.Command("sudo", "systemctl", "stop", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "darwin":
			cmd := exec.Command("brew", "services", "stop", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		case "windows":
			cmd := exec.Command("wsl", "-d", "Ubuntu", "--", "sudo", "systemctl", "stop", "k3s")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()

		default:
			return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		}
	})
}

// NewK3sCommand creates the k3s management command
func NewK3sCommand() *cobra.Command {
	manager := &K3sManager{}
	cmd := &cobra.Command{
		Use:   "k3s",
		Short: "Manage k3s Kubernetes cluster",
		Long:  `Commands to install, start, stop, and uninstall k3s Kubernetes clusters across supported platforms.`,
	}

	commands := []struct {
		use   string
		short string
		runE  func(*cobra.Command, []string) error
	}{
		{"install", "Install k3s Kubernetes cluster", func(_ *cobra.Command, _ []string) error { return manager.Install() }},
		{"uninstall", "Uninstall k3s Kubernetes cluster", func(_ *cobra.Command, _ []string) error { return manager.Uninstall() }},
		{"start", "Start k3s Kubernetes cluster", func(_ *cobra.Command, _ []string) error { return manager.Start() }},
		{"stop", "Stop k3s Kubernetes cluster", func(_ *cobra.Command, _ []string) error { return manager.Stop() }},
	}

	for _, c := range commands {
		subCmd := &cobra.Command{
			Use:   c.use,
			Short: c.short,
			Args:  cobra.NoArgs,
			RunE:  c.runE,
		}
		cmd.AddCommand(subCmd)
	}

	return cmd
}
