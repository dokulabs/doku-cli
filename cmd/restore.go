package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/dokulabs/doku-cli/internal/backup"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	restoreInstance  string
	restoreOverwrite bool
	restoreEnvOnly   bool
	restoreYes       bool
)

var restoreCmd = &cobra.Command{
	Use:   "restore <backup-file>",
	Short: "Restore a service from backup",
	Long: `Restore a service's data and configuration from a backup file.

This will restore:
  - Environment files to ~/.doku/services/
  - Volume metadata (volumes need to be recreated by reinstalling)

After restoring, you can reinstall the service to reuse the restored data:
  doku install <service>

Examples:
  doku restore postgres-20240101-120000.tar.gz
  doku restore ./my-backup.tar --instance mypostgres
  doku restore backup.tar.gz --overwrite  # Overwrite existing files
  doku restore backup.tar.gz --env-only   # Only restore env files`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().StringVarP(&restoreInstance, "instance", "i", "", "Target instance name (defaults to original)")
	restoreCmd.Flags().BoolVar(&restoreOverwrite, "overwrite", false, "Overwrite existing files")
	restoreCmd.Flags().BoolVar(&restoreEnvOnly, "env-only", false, "Only restore environment files")
	restoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "Skip confirmation prompt")
}

func runRestore(cmd *cobra.Command, args []string) error {
	backupPath := args[0]

	// Resolve path
	if !filepath.IsAbs(backupPath) {
		// Check if it's just a filename (look in backup dir)
		cfgMgr, err := config.New()
		if err != nil {
			return fmt.Errorf("failed to create config manager: %w", err)
		}

		backupDir := filepath.Join(cfgMgr.GetDokuDir(), "backups")
		possiblePath := filepath.Join(backupDir, backupPath)

		if _, err := os.Stat(possiblePath); err == nil {
			backupPath = possiblePath
		} else {
			// Try as relative path from current directory
			absPath, err := filepath.Abs(backupPath)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}
			backupPath = absPath
		}
	}

	// Check if backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupPath)
	}

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create Docker client
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Create backup manager
	backupMgr := backup.NewManager(dockerClient, cfgMgr)

	// Get backup info
	info, err := backupMgr.GetBackupInfo(backupPath)
	if err != nil {
		color.Yellow("Warning: Could not read backup metadata: %v", err)
		info = &backup.BackupInfo{
			Name: filepath.Base(backupPath),
			Path: backupPath,
		}
	}

	// Determine target instance name
	targetInstance := restoreInstance
	if targetInstance == "" {
		targetInstance = info.InstanceName
	}

	if targetInstance == "" {
		return fmt.Errorf("could not determine target instance name. Use --instance to specify")
	}

	// Show backup info
	fmt.Println()
	color.Cyan("Restore Backup")
	fmt.Println()

	fmt.Printf("  Backup file: %s\n", filepath.Base(backupPath))
	if info.ServiceType != "" {
		fmt.Printf("  Service type: %s\n", info.ServiceType)
	}
	if info.Version != "" {
		fmt.Printf("  Version: %s\n", info.Version)
	}
	fmt.Printf("  Target instance: %s\n", targetInstance)

	if len(info.EnvFiles) > 0 {
		fmt.Printf("  Env files to restore: %d\n", len(info.EnvFiles))
	}
	if len(info.Volumes) > 0 {
		fmt.Printf("  Volumes referenced: %d\n", len(info.Volumes))
	}

	fmt.Println()

	// Confirmation
	if !restoreYes {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with restore?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			color.Yellow("Restore cancelled")
			return nil
		}
		fmt.Println()
	}

	// Prepare restore options
	opts := backup.RestoreOptions{
		BackupPath:     backupPath,
		InstanceName:   targetInstance,
		RestoreVolumes: !restoreEnvOnly,
		RestoreEnv:     true,
		Overwrite:      restoreOverwrite,
	}

	// Perform restore
	color.Cyan("Restoring backup...")
	fmt.Println()

	result, err := backupMgr.Restore(opts)
	if err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	// Show results
	fmt.Println()
	color.Green("Restore completed!")
	fmt.Println()

	if len(result.RestoredEnvFiles) > 0 {
		fmt.Printf("  Restored env files: %d\n", len(result.RestoredEnvFiles))
		for _, f := range result.RestoredEnvFiles {
			fmt.Printf("    - %s\n", f)
		}
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		color.Yellow("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Println()
	color.New(color.Faint).Println("Next steps:")
	if info.ServiceType != "" && info.ServiceType != "custom-project" {
		color.New(color.Faint).Printf("  1. Install the service: doku install %s\n", info.ServiceType)
	} else {
		color.New(color.Faint).Println("  1. Install or deploy your service")
	}
	color.New(color.Faint).Println("  2. The restored env files will be used automatically")
	fmt.Println()

	return nil
}
