package sds

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stratosnet/sds/framework/utils"
	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
	ppns "github.com/stratosnet/sds/pp/namespace"
	"github.com/stretchr/testify/assert"
)

func randomFileName(size int, ext string) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x.%s", b, ext), nil
}

// TestPluginLoad smoke tsting if plugin successfully compiled and work on current OS
func TestRPC_GetOzone(t *testing.T) {
	utils.NewDefaultLogger("/", true, false)
	wallet, err := NewSdsSecp256k1Wallet("0xf4a2b939592564feb35ab10a8e04f6f2fe0943579fb3c9c33505298978b74893")
	assert.Equal(t, err, nil)
	rpc, err := NewRpc("http://0.0.0.0:18281")
	assert.Equal(t, err, nil)
	addr := wallet.GetAddress()
	fmt.Println("addr", addr)
	oz, err := rpc.GetOzone(wallet)
	assert.Equal(t, err, nil)

	fmt.Println("ozone", oz.Ozone)
	fmt.Println("seq", oz.SequenceNumber)

	fileName, err := randomFileName(16, "txt")
	assert.Equal(t, err, nil)

	fileData := make([]byte, ppns.FILE_DATA_SAFE_SIZE+1)
	fileHash := CreateFileHash(fileData)
	fmt.Println("file hash", fileHash)

	_, err = rand.Read(fileData)
	assert.Equal(t, err, nil)

	res, err := rpc.RequestUpload(wallet, oz.SequenceNumber, fileName, fileHash, len(fileData))
	fmt.Println("-> request upload", res)
	fmt.Println("res", res)
	fmt.Println("res err", err)
	assert.Equal(t, err, nil)
	assert.Equal(t, res.Return, "1")
	fmt.Println("offset start", *res.OffsetStart)
	fmt.Println("offset end", *res.OffsetEnd)
	fmt.Println("total file length", len(fileData))

	for res.Return == rpc_api.UPLOAD_DATA {
		chunkData := make([]byte, *res.OffsetEnd-*res.OffsetStart)
		copy(chunkData, fileData[*res.OffsetStart:*res.OffsetEnd])
		fmt.Println("chunk file length", len(chunkData))
		assert.Equal(t, err, nil)
		fileChunk := base64.StdEncoding.EncodeToString(chunkData)
		assert.Equal(t, err, nil)

		res, err = rpc.UploadData(wallet, oz.SequenceNumber, fileHash, fileChunk)
		fmt.Println("-> upload data", res)
		fmt.Println("res", res)
		fmt.Println("res err", err)
		assert.Equal(t, err, nil)
	}
	assert.Equal(t, res.Return, rpc_api.SUCCESS)
}
