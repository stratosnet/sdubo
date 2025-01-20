package config

import (
	"encoding/hex"

	fwsecp256k1 "github.com/stratosnet/sds/framework/crypto/secp256k1"
)

type Sds struct {
	// Enabled is used to switch on/off sds uploading and downloading part
	Enabled bool
	// PrivateKey is the secret that will be used to sign uploading file to SDS (hex value, 0x not required)
	PrivateKey string
	// RPC for pp node (where it will be uploaded/dowloaded), multiaddr format
	RPC string
}

func sdsConfig() Sds {
	w, _ := fwsecp256k1.GenerateKey()
	pkStr := "0x" + hex.EncodeToString(w.Bytes())

	return Sds{
		Enabled:    false,
		PrivateKey: pkStr,
		RPC:        "/ip4/127.0.0.1/tcp/18281/http",
	}
}
