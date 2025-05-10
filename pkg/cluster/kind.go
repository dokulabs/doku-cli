package cluster

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

type KindManager struct{}

func (k *KindManager) IsInstalled() bool {
	_, err := exec.LookPath("kind")
	return err == nil
}

func (k *KindManager) Install() error {
	fmt.Println("Follow https://kind.sigs.k8s.io/docs/user/quick-start/ for installation instructions.")
	return nil
}

func (k *KindManager) Uninstall() error {
	fmt.Println("Manual deletion of kind clusters may be required.")
	return nil
}

func (k *KindManager) IsRunning() bool {
	cmd := exec.Command("docker", "ps", "--filter", "name=kind", "--format", "{{.Names}}")
	output, err := cmd.Output()
	return err == nil && len(output) > 0
}

func (k *KindManager) Start() error {
	cmd := exec.Command("kind", "create", "cluster")
	return cmd.Run()
}

func (k *KindManager) Stop() error {
	cmd := exec.Command("kind", "delete", "cluster")
	return cmd.Run()
}

func NewKindCommand() *cobra.Command {
	manager := &KindManager{}
	cmd := &cobra.Command{
		Use:   "kind",
		Short: "Manage Kind cluster",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install Kind",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Install()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Kind",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Uninstall()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start Kind cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Start()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop Kind cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Stop()
		},
	})

	return cmd
}
