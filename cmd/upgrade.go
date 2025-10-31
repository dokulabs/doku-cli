package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	githubAPIURL = "https://api.github.com/repos/dokulabs/doku-cli/releases/latest"
	repoURL      = "https://github.com/dokulabs/doku-cli"
)

var (
	upgradeForce bool
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade doku to the latest version",
	Long: `Upgrade doku CLI to the latest version from GitHub releases.

This command will:
  • Check for the latest version available
  • Download the appropriate binary for your platform
  • Replace the current binary with the new version
  • Verify the installation

Use --force to skip confirmation prompt.`,
	RunE: runUpgrade,
}

func init() {
	selfCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().BoolVarP(&upgradeForce, "force", "f", false, "Force upgrade without confirmation")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	fmt.Println()
	color.New(color.Bold, color.FgCyan).Println("Doku Self-Upgrade")
	fmt.Println()

	// Get current version
	currentVersion := version
	if currentVersion == "" || currentVersion == "dev" {
		color.Yellow("⚠️  Development build detected")
		fmt.Println()

		if !upgradeForce {
			proceed := false
			prompt := &survey.Confirm{
				Message: "This appears to be a development build. Continue with upgrade?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &proceed); err != nil {
				return fmt.Errorf("confirmation failed: %w", err)
			}
			if !proceed {
				color.Yellow("Upgrade cancelled")
				return nil
			}
		}
		currentVersion = "unknown"
	}

	fmt.Printf("Current version: %s\n", color.CyanString(currentVersion))
	fmt.Println()

	// Check for latest version
	fmt.Println("Checking for latest version...")

	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	fmt.Printf("Latest version:  %s\n", color.GreenString(latestVersion))
	fmt.Println()

	// Compare versions
	if currentVersion != "unknown" && currentVersion != "dev" {
		currentClean := strings.TrimPrefix(currentVersion, "v")
		if currentClean == latestVersion {
			color.Green("✓ You are already running the latest version!")
			return nil
		}
	}

	// Determine platform and architecture
	platform := runtime.GOOS
	arch := runtime.GOARCH

	binaryName := fmt.Sprintf("doku-%s-%s", platform, arch)
	if platform == "windows" {
		binaryName += ".exe"
	}

	// Find the download URL
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary found for platform: %s/%s\nAvailable at: %s/releases", platform, arch, repoURL)
	}

	// Confirm upgrade
	if !upgradeForce {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Upgrade to version %s?", latestVersion),
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !confirm {
			color.Yellow("Upgrade cancelled")
			return nil
		}
	}

	fmt.Println()
	fmt.Printf("Downloading %s...\n", color.CyanString(binaryName))

	// Download the new binary
	tmpFile, err := downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.Remove(tmpFile)

	color.Green("✓ Download complete")

	// Get the path of the current binary
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	fmt.Printf("Installing to: %s\n", execPath)

	// Make the new binary executable
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	// Replace the old binary with the new one
	// Create a backup first
	backupPath := execPath + ".bak"
	if err := copyFile(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Replace the binary
	if err := copyFile(tmpFile, execPath); err != nil {
		// Restore backup on failure
		copyFile(backupPath, execPath)
		os.Remove(backupPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	// Verify the installation
	fmt.Println()
	fmt.Println("Verifying installation...")

	verifyCmd := exec.Command(execPath, "version")
	output, err := verifyCmd.Output()
	if err != nil {
		color.Yellow("⚠️  Could not verify installation")
	} else {
		fmt.Println(string(output))
	}

	// Success
	fmt.Println()
	color.Green("✓ Upgrade completed successfully!")
	fmt.Println()
	color.New(color.Bold).Printf("Doku has been upgraded to version %s\n", latestVersion)
	fmt.Println()

	return nil
}

func getLatestRelease() (*GitHubRelease, error) {
	resp, err := http.Get(githubAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	return &release, nil
}

func downloadBinary(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "doku-upgrade-*")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Copy the response body to the file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}
