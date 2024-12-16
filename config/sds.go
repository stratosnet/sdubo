package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	ma "github.com/multiformats/go-multiaddr"
	fwsecp256k1 "github.com/stratosnet/sds/framework/crypto/secp256k1"
)

type Sds struct {
	// Enabled is used to switch on/off sds uploading and downloading part
	Enabled bool
	// PrivateKey is the secret that will be used to sign uploading file to SDS (hex value, 0x not required)
	PrivateKey string
	// SUMKey (Spfs user management key) is a secret for folder hashing for user management
	SUMKey string
	// RPC for pp node (where it will be uploaded/dowloaded), multiaddr format
	RPC string
}

func (c *Sds) GetRpcAddress() (string, error) {
	addr, err := ma.NewMultiaddr(c.RPC)
	if err != nil {
		return "", err
	}

	var ip, port, protocol string

	components := ma.Split(addr)
	for _, c := range components {
		comp := c.(*ma.Component)
		switch comp.Protocol().Name {
		case "ip4", "ip6":
			ip = comp.Value()
		case "tcp":
			port = comp.Value()
		case "http", "https":
			protocol = comp.Protocol().Name
		}
	}

	if ip == "" || port == "" || protocol == "" {
		return "", fmt.Errorf("multiaddr must contain both ip and tcp and http")
	}

	url := fmt.Sprintf("%s://%s:%s", protocol, ip, port)
	return url, nil
}

func sdsConfig() Sds {
	w, _ := fwsecp256k1.GenerateKey()
	pkStr := "0x" + hex.EncodeToString(w.Bytes())

	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		panic("failed to create secret for SUMKey")
	}
	sumKey := hex.EncodeToString(secret)
	return Sds{
		Enabled:    false,
		PrivateKey: pkStr,
		SUMKey:     sumKey,
		RPC:        "/ip4/127.0.0.1/tcp/18281/http",
	}
}
