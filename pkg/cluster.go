package pkg

import (
	"fmt"
	"github.com/dokulabs/doku/pkg/cluster"
)

const (
	Minikube = "minikube"
	//K3s      = "k3s"
	//Kind     = "kind"
)

func GetClusterManager(spinner *Spinner) (cluster.ClusterManager, error) {
	cfg, err := ReadConfig(spinner)
	if err != nil {
		spinner.Error("error loading config: %v", err)
	}
	provider := cfg.Dist
	switch provider {
	case Minikube:
		return &cluster.MinikubeManager{}, nil
	//case K3s:
	//	return &K3sManager{}, nil
	//case Kind:
	//	return &KindManager{}, nil
	default:
		return nil, fmt.Errorf("unsupported cluster provider: %s", provider)
	}
}
