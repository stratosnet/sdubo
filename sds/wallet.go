package sds

import (
	"encoding/hex"
	"fmt"
	"time"

	fwsecp256k1 "github.com/stratosnet/sds/framework/crypto/secp256k1"
	fwcryptotypes "github.com/stratosnet/sds/framework/crypto/types"
	fwtypes "github.com/stratosnet/sds/framework/types"
	msgutils "github.com/stratosnet/sds/sds-msg/utils"
)

type SdsWallet struct {
	privateKey fwcryptotypes.PrivKey
}

func GenerateSdsWallet() (*SdsWallet, error) {
	pk, err := fwsecp256k1.GenerateKey()
	if err != nil {
		return nil, err
	}
	return NewSdsWallet(pk.String())
}

func NewSdsWallet(privatKey string) (*SdsWallet, error) {
	if len(privatKey) < 2 {
		return nil, fmt.Errorf("wrong pk length")
	}
	if privatKey[:2] == "0x" {
		privatKey = privatKey[2:]
	}
	pkBytes, err := hex.DecodeString(privatKey)
	if err != nil {
		return nil, err
	}
	return &SdsWallet{
		privateKey: fwsecp256k1.Generate(pkBytes),
	}, nil
}

func (w *SdsWallet) GetAddress() string {
	return fwtypes.WalletAddress(w.privateKey.PubKey().Address()).String()
}

func (w *SdsWallet) GetBech32PubKey() (string, error) {
	wpk, err := fwtypes.WalletPubKeyToBech32(w.privateKey.PubKey())
	if err != nil {
		return "", err
	}
	return wpk, nil
}

func (w *SdsWallet) SignFileUpload(sn, fileHash string) ([]byte, error) {
	nowSec := time.Now().Unix()
	sign, err := w.privateKey.Sign([]byte(msgutils.GetFileUploadWalletSignMessage(fileHash, w.GetAddress(), sn, nowSec)))
	if err != nil {
		return nil, err
	}
	return sign, nil
}

func (w *SdsWallet) SignDownloadData(sn, fileHash string) ([]byte, error) {
	nowSec := time.Now().Unix()
	sign, err := w.privateKey.Sign([]byte(msgutils.GetFileDownloadWalletSignMessage(fileHash, w.GetAddress(), sn, nowSec)))
	if err != nil {
		return nil, err
	}
	return sign, nil
}
