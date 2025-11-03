package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var monitorPrintOnly bool

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Open monitoring dashboard",
	Long: `Open the configured monitoring dashboard (SignOz or Sentry) in your browser.

This provides a unified interface to view logs, errors, traces, and performance
metrics from all your Doku services in one place.

Examples:
  doku monitor              # Open monitoring dashboard in browser
  doku monitor --print      # Print dashboard URL without opening`,
	RunE: runMonitor,
}

func init() {
	rootCmd.AddCommand(monitorCmd)
	monitorCmd.Flags().BoolVar(&monitorPrintOnly, "print", false, "Print URL instead of opening browser")
}

func runMonitor(cmd *cobra.Command, args []string) error {
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

	// Get config
	cfg, err := cfgMgr.Get()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Check if monitoring is configured
	if !cfg.Monitoring.Enabled || cfg.Monitoring.Tool == "none" {
		color.Yellow("⚠️  Monitoring is not configured.")
		fmt.Println()
		fmt.Println("To set up monitoring, reinitialize Doku:")
		fmt.Println("  doku init")
		fmt.Println()
		fmt.Println("Or install a monitoring tool manually:")
		fmt.Println("  doku install signoz    # For full observability")
		fmt.Println("  doku install sentry    # For error tracking")
		return nil
	}

	// Get monitoring URL
	monitoringURL := cfg.Monitoring.URL
	if monitoringURL == "" {
		return fmt.Errorf("monitoring URL not configured")
	}

	// Get tool display name
	toolName := "Monitoring"
	switch cfg.Monitoring.Tool {
	case "signoz":
		toolName = "SignOz"
	case "sentry":
		toolName = "Sentry"
	}

	// Print or open
	if monitorPrintOnly {
		fmt.Printf("%s Dashboard: %s\n", toolName, monitoringURL)
		return nil
	}

	// Open in browser
	color.Cyan("Opening %s dashboard...", toolName)
	if err := openBrowser(monitoringURL); err != nil {
		color.Yellow("⚠️  Could not open browser automatically")
		fmt.Println()
		fmt.Printf("%s Dashboard: %s\n", toolName, monitoringURL)
		fmt.Println()
		color.New(color.Faint).Println("Tip: Copy the URL above and paste it into your browser")
		return nil
	}

	color.Green("✓ %s dashboard opened in browser", toolName)
	fmt.Println()
	color.New(color.Faint).Printf("Dashboard: %s\n", monitoringURL)
	fmt.Println()

	// Show quick tips
	color.New(color.Faint).Println("💡 Tips:")
	color.New(color.Faint).Println("  • View logs, traces, and metrics from all services")
	color.New(color.Faint).Println("  • Set up alerts for critical errors")
	color.New(color.Faint).Println("  • Analyze service performance and dependencies")

	return nil
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
