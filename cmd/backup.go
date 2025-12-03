package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/dokulabs/doku-cli/internal/backup"
	"github.com/dokulabs/doku-cli/internal/config"
	"github.com/dokulabs/doku-cli/internal/docker"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	backupOutput     string
	backupNoCompress bool
	backupEnvOnly    bool
)

var backupCmd = &cobra.Command{
	Use:   "backup <service>",
	Short: "Backup a service's data and configuration",
	Long: `Create a backup of a service's Docker volumes and environment files.

The backup includes:
  - Environment files (~/.doku/services/<service>.env)
  - Docker volume metadata
  - Service configuration

Backups are stored in ~/.doku/backups/ by default.

Examples:
  doku backup postgres                    # Backup postgres service
  doku backup postgres -o ./my-backup.tar # Custom output path
  doku backup postgres --env-only         # Only backup env files
  doku backup postgres --no-compress      # Create uncompressed tar`,
	Args: cobra.ExactArgs(1),
	RunE: runBackup,
}

var backupListCmd = &cobra.Command{
	Use:   "list [service]",
	Short: "List available backups",
	Long: `List all available backups, optionally filtered by service name.

Examples:
  doku backup list              # List all backups
  doku backup list postgres     # List backups for postgres only`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBackupList,
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.AddCommand(backupListCmd)

	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "Output path for backup file")
	backupCmd.Flags().BoolVar(&backupNoCompress, "no-compress", false, "Create uncompressed tar archive")
	backupCmd.Flags().BoolVar(&backupEnvOnly, "env-only", false, "Only backup environment files (skip volumes)")
}

func runBackup(cmd *cobra.Command, args []string) error {
	instanceName := args[0]

	// Create config manager
	cfgMgr, err := config.New()
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Check if initialized
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

	// Create backup manager
	backupMgr := backup.NewManager(dockerClient, cfgMgr)

	// Prepare backup options
	opts := backup.BackupOptions{
		InstanceName:   instanceName,
		OutputPath:     backupOutput,
		IncludeVolumes: !backupEnvOnly,
		IncludeEnv:     true,
		Compress:       !backupNoCompress,
	}

	fmt.Println()
	color.Cyan("Creating backup for '%s'...", instanceName)
	fmt.Println()

	// Create backup
	info, err := backupMgr.Backup(opts)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Show success message
	fmt.Println()
	color.Green("Backup created successfully!")
	fmt.Println()

	fmt.Printf("  File: %s\n", info.Path)
	fmt.Printf("  Size: %s\n", formatBytes(info.Size))

	if len(info.EnvFiles) > 0 {
		fmt.Printf("  Env files: %d\n", len(info.EnvFiles))
		for _, f := range info.EnvFiles {
			fmt.Printf("    - %s\n", f)
		}
	}

	if len(info.Volumes) > 0 {
		fmt.Printf("  Volumes: %d\n", len(info.Volumes))
		for _, v := range info.Volumes {
			fmt.Printf("    - %s\n", v)
		}
	}

	fmt.Println()
	color.New(color.Faint).Println("Restore with:")
	color.New(color.Faint).Printf("  doku restore %s\n", info.Path)
	fmt.Println()

	return nil
}

func runBackupList(cmd *cobra.Command, args []string) error {
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

	var instanceName string
	if len(args) > 0 {
		instanceName = args[0]
	}

	fmt.Println()

	if instanceName != "" {
		// List backups for specific instance
		backups, err := backupMgr.ListBackups(instanceName)
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}

		if len(backups) == 0 {
			color.Yellow("No backups found for '%s'", instanceName)
			fmt.Println()
			return nil
		}

		color.Cyan("Backups for '%s':", instanceName)
		fmt.Println()
		printBackupTable(backups)
	} else {
		// List all backups by scanning directory
		backupDir := backupMgr.GetBackupDir()
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			color.Yellow("No backups found")
			fmt.Println()
			return nil
		}

		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return fmt.Errorf("failed to read backup directory: %w", err)
		}

		var allBackups []backup.BackupInfo
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !isBackupFile(name) {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Try to get more info from the backup
			backupPath := filepath.Join(backupDir, name)
			backupInfo, err := backupMgr.GetBackupInfo(backupPath)
			if err != nil {
				// Use basic info if metadata read fails
				backupInfo = &backup.BackupInfo{
					Name:      name,
					Path:      backupPath,
					CreatedAt: info.ModTime(),
					Size:      info.Size(),
				}
			}

			allBackups = append(allBackups, *backupInfo)
		}

		if len(allBackups) == 0 {
			color.Yellow("No backups found")
			fmt.Println()
			return nil
		}

		color.Cyan("Available Backups:")
		fmt.Println()
		printBackupTable(allBackups)
	}

	fmt.Println()
	return nil
}

func printBackupTable(backups []backup.BackupInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tSERVICE\tSIZE\tCREATED\n")
	fmt.Fprintf(w, "----\t-------\t----\t-------\n")

	for _, b := range backups {
		serviceName := b.ServiceType
		if serviceName == "" {
			serviceName = b.InstanceName
		}

		created := b.CreatedAt.Format("2006-01-02 15:04")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.Name, serviceName, formatBytes(b.Size), created)
	}

	w.Flush()
}

func isBackupFile(name string) bool {
	return filepath.Ext(name) == ".tar" ||
		(len(name) > 7 && name[len(name)-7:] == ".tar.gz")
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
