package service

type Backend interface {
	// Watch for changes in the backend and return where there is a change.
	Watch() error
	// Load all registered services
	Services() ([]ServiceRegistration, error)
}

type ServiceRegistration struct {
	Name     string
	Backends []string
}
