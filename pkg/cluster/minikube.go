package cluster

import (
	"fmt"
	"os/exec"
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
	cmd := exec.Command("minikube", "start", "--driver=docker")
	return cmd.Run()
}

func (m *MinikubeManager) Stop() error {
	cmd := exec.Command("minikube", "stop")
	return cmd.Run()
}

func (m *MinikubeManager) Status() error {
	cmd := exec.Command("minikube", "status")
	return cmd.Run()
}
