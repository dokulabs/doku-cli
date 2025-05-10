package cluster

type ClusterManager interface {
	IsInstalled() bool
	Install() error
	Uninstall() error
	IsRunning() bool
	Start() error
	Stop() error
}
