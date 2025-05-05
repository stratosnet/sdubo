package config

type Sg struct {
	// Enabled is used to switch on/off sg reports
	Enabled bool
	// URI for sg api
	URI string
}

func sgConfig() Sg {
	return Sg{
		Enabled: false,
		URI:     "http://127.0.0.1:8000",
	}
}
