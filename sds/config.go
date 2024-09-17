package sds

type Config struct {
	privateKey string
	sdsRpcURL  string
}

func NewConfig(privateKey string, sdsRpcURL string) *Config {
	return &Config{
		privateKey: privateKey,
		sdsRpcURL:  sdsRpcURL,
	}
}
