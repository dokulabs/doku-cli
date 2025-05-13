package cluster

import (
	"fmt"
	"os/exec"
)

type MinikubeManager struct{}

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
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	return err
}

func (m *MinikubeManager) Name() string {
	return "minikube"
}
