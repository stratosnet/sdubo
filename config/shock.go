package config

type mfsConn struct {
	// DSN is the data source name of connector to the cluster storage
	DSN string
}

type Shock struct {
	MFS *mfsConn
}

func shockConfig() Shock {
	return Shock{
		MFS: nil,
	}
}
