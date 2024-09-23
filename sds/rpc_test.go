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

func TestRPC_Upload(t *testing.T) {
	utils.NewDefaultLogger("/", true, false)
	wallet, err := NewSdsWallet("0xf4a2b939592564feb35ab10a8e04f6f2fe0943579fb3c9c33505298978b74893")
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

func TestRPC_Download(t *testing.T) {
	utils.NewDefaultLogger("/", true, false)
	wallet, err := NewSdsWallet("0xf4a2b939592564feb35ab10a8e04f6f2fe0943579fb3c9c33505298978b74893")
	assert.Equal(t, err, nil)
	rpc, err := NewRpc("http://0.0.0.0:18281")
	assert.Equal(t, err, nil)
	addr := wallet.GetAddress()
	fmt.Println("addr", addr)
	oz, err := rpc.GetOzone(wallet)
	assert.Equal(t, err, nil)

	fmt.Println("ozone", oz.Ozone)
	fmt.Println("seq", oz.SequenceNumber)

	fileHash := "v05j1m517ljekhi1c4ce82pb62c5p1vdjvrbph2g"
	// fileHash := "v05j1m556nt0igqi8f76625t9sn4e13vpgr0mi0o"

	res, err := rpc.RequestDownload(wallet, oz.SequenceNumber, fileHash)
	fmt.Println("-> request download", res)
	fmt.Printf("res: %+v\n", res)
	fmt.Println("res err", err)
	assert.Equal(t, err, nil)

	var (
		fileSize uint64 = 0
		fileData        = make([]byte, 0)
	)

	// Handle result:1 sending the content
	for res.Return == rpc_api.DOWNLOAD_OK || res.Return == rpc_api.DL_OK_ASK_INFO {
		if res.Return == rpc_api.DL_OK_ASK_INFO {
			fmt.Println("- received response (return: DL_OK_ASK_INFO)")
			res, err = rpc.DownloadedFileInfo(wallet, res.ReqId, fileHash, fileSize)
			fmt.Println("- request file information verification (method: user_downloadedFileInfo)")
			fmt.Println("-> download file info", res)
			fmt.Println("res", res)
			fmt.Println("res err", err)
			assert.Equal(t, err, nil)
		} else {
			fmt.Println("- received response (return: DOWNLOAD_OK)")
			start := *res.OffsetStart
			end := *res.OffsetEnd
			fileSize = fileSize + (end - start)
			decoded, _ := base64.StdEncoding.DecodeString(res.FileData)
			fileData = append(fileData, decoded...)
			fmt.Println("- fileData", fileData)
			fmt.Println("- fileSize", fileSize)
			fmt.Println("- fileHash", fileHash)
			fmt.Println("- req id", res.ReqId)
			res, err = rpc.DownloadData(wallet, res.ReqId, fileHash)
			fmt.Println("- request downloading file data (method: user_downloadData)")
			fmt.Println("-> download data", res)
			fmt.Println("res", res)
			fmt.Println("res err", err)
			assert.Equal(t, err, nil)
		}
	}
	assert.Equal(t, res.Return, rpc_api.SUCCESS)
}
