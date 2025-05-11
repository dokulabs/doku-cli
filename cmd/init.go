package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// Constants for configuration
const (
	configDirPerm  = 0755
	configFilePerm = 0644
	configDir      = ".doku"
	configFile     = "cluster_config"
)

// Distribution configuration
type Distribution struct {
	Name        string
	Description string
	Manager     ClusterManager
}

var distributions = []Distribution{
	{
		Name:        "k3s",
		Description: "Lightweight Kubernetes distribution",
		Manager:     &K3sManager{},
	},
	{
		Name:        "minikube",
		Description: "Local Kubernetes, focusing on ease of use",
		Manager:     &MinikubeManager{},
	},
	{
		Name:        "kind",
		Description: "Kubernetes IN Docker for running local clusters",
		Manager:     &KindManager{},
	},
}

func ManageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage Kubernetes clusters",
		Long:  `A CLI tool to initialize, start, stop, and uninstall Kubernetes clusters.`,
	}

	cmd.AddCommand(NewInitCommand())
	cmd.AddCommand(newClusterActionCommand("install"))
	cmd.AddCommand(newClusterActionCommand("start"))
	cmd.AddCommand(newClusterActionCommand("stop"))
	cmd.AddCommand(newClusterActionCommand("uninstall"))

	return cmd
}

func NewInitCommand() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a Kubernetes cluster",
		Long:  `Select and configure a Kubernetes distribution for cluster management.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if already initialized
			if !force {
				if _, err := loadSelectedDistribution(); err == nil {
					return fmt.Errorf("cluster already initialized; use --force to reinitialize")
				}
			}

			fmt.Println("Available Kubernetes distributions:")
			for i, dist := range distributions {
				fmt.Printf("[%d] %s - %s\n", i+1, dist.Name, dist.Description)
			}

			choice, err := promptUserSelection(len(distributions))
			if err != nil {
				return err
			}

			selected := distributions[choice-1]
			if err := saveSelectedDistribution(selected.Name); err != nil {
				return fmt.Errorf("failed to save selection: %w", err)
			}

			fmt.Printf("Installing %s...\n", selected.Name)
			return selected.Manager.Install()
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force reinitialization of cluster")
	return cmd
}

func newClusterActionCommand(action string) *cobra.Command {
	return &cobra.Command{
		Use:   action,
		Short: fmt.Sprintf("%s the selected Kubernetes cluster", strings.Title(action)),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := getClusterManager()
			if err != nil {
				return err
			}

			switch action {
			case "install":
				return manager.Install()
			case "start":
				return manager.Start()
			case "stop":
				return manager.Stop()
			case "uninstall":
				return manager.Uninstall()
			default:
				return fmt.Errorf("unsupported action: %s", action)
			}
		},
	}
}

// Helper functions
func promptUserSelection(max int) (int, error) {
	fmt.Printf("Select an option [1-%d]: ", max)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return 0, fmt.Errorf("failed to read input")
	}

	choiceStr := strings.TrimSpace(scanner.Text())
	choice, err := strconv.Atoi(choiceStr)
	if err != nil || choice < 1 || choice > max {
		return 0, fmt.Errorf("invalid choice: %s", choiceStr)
	}

	return choice, nil
}

func getClusterManager() (ClusterManager, error) {
	distName, err := loadSelectedDistribution()
	if err != nil {
		return nil, fmt.Errorf("could not load selected distribution: %w", err)
	}

	for _, dist := range distributions {
		if dist.Name == distName {
			return dist.Manager, nil
		}
	}
	return nil, fmt.Errorf("unsupported distribution: %s", distName)
}

func configFilePath() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	dokuDir := filepath.Join(u.HomeDir, configDir)
	if err := os.MkdirAll(dokuDir, configDirPerm); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return filepath.Join(dokuDir, configFile), nil
}

func saveSelectedDistribution(name string) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(name+"\n"), configFilePerm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func loadSelectedDistribution() (string, error) {
	path, err := configFilePath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}
