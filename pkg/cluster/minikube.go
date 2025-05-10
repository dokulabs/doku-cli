package cluster

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

type MinikubeManager struct{}

func (m *MinikubeManager) IsInstalled() bool {
	_, err := exec.LookPath("minikube")
	return err == nil
}

func (m *MinikubeManager) Install() error {
	fmt.Println("Follow https://minikube.sigs.k8s.io/docs/start/ for installation.")
	return nil
}

func (m *MinikubeManager) Uninstall() error {
	cmd := exec.Command("minikube", "delete")
	return cmd.Run()
}

func (m *MinikubeManager) IsRunning() bool {
	cmd := exec.Command("minikube", "status", "--format", "{{.Host}}")
	output, err := cmd.Output()
	return err == nil && string(output) == "Running"
}

func (m *MinikubeManager) Start() error {
	cmd := exec.Command("minikube", "start")
	return cmd.Run()
}

func (m *MinikubeManager) Stop() error {
	cmd := exec.Command("minikube", "stop")
	return cmd.Run()
}

func NewMinikubeCommand() *cobra.Command {
	manager := &MinikubeManager{}
	cmd := &cobra.Command{
		Use:   "minikube",
		Short: "Manage Minikube cluster",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Install Minikube",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Install()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Minikube",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Uninstall()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: "Start Minikube",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Start()
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop Minikube",
		RunE: func(cmd *cobra.Command, args []string) error {
			return manager.Stop()
		},
	})

	return cmd
}
