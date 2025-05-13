package pkg

import (
	"os/exec"
)

func IsDockerRunning() bool {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	return err == nil
}
