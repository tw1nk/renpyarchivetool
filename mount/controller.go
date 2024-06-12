package mount

type Controller interface {
	MountPath() string
	Unmount() error
	Done() <-chan struct{}
}
