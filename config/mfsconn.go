package config

type MfsConn struct {
	// DSN is the data source name of connector to the cluster storage
	DSN string
}

func mfsConnConfig() MfsConn {
	return MfsConn{
		DSN: "native",
	}
}
