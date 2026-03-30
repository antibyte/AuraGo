package setup

// HTTPSConfig holds HTTPS/network configuration chosen during setup.
type HTTPSConfig struct {
	Enabled bool
	Domain  string
	Email   string
	// BindAll means bind to 0.0.0.0 (LAN access) instead of 127.0.0.1
	BindAll bool
}
