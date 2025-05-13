package cluster

type ClusterManager interface {
	IsRunning() bool
	Start() error
	Stop() error
	Status() error
	Name() string
}
