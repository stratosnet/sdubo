package sds

import (
	"encoding/base64"
	"fmt"
	"strings"

	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
)

type Fetcher struct {
	cfg    *Config
	wallet *SdsWallet
	rpc    *Rpc
}

func NewFetcher(cfg *Config) (*Fetcher, error) {
	wallet, err := NewSdsSecp256k1Wallet(cfg.privateKey)
	if err != nil {
		return nil, err
	}
	rpc, err := NewRpc(cfg.sdsRpcURL)
	if err != nil {
		return nil, err
	}

	return &Fetcher{
		cfg:    cfg,
		wallet: wallet,
		rpc:    rpc,
	}, nil
}

func isDublErr(ret string) bool {
	// this is sp error, means that file already exist and uploaded, so we could just link
	return strings.Contains(ret, "Same file with the name")
}

func (f *Fetcher) Upload(fileData []byte) (string, error) {
	fileHash := CreateFileHash(fileData)

	oz, err := f.rpc.GetOzone(f.wallet)
	if err != nil {
		return "", err
	}

	// TODO: How to get file name?
	fileName, err := randomFileName(16, "txt")
	if err != nil {
		return "", err
	}

	res, err := f.rpc.RequestUpload(f.wallet, oz.SequenceNumber, fileName, fileHash, len(fileData))
	if err != nil {
		if isDublErr(err.Error()) {
			return fileHash, nil
		}
		return "", err
	}
	if res.Return != rpc_api.UPLOAD_DATA {
		if isDublErr(res.Return) {
			return fileHash, nil
		}
		return "", fmt.Errorf("failed sp request upload with error: %s", res.Return)
	}

	for res.Return == rpc_api.UPLOAD_DATA {
		chunkData := make([]byte, *res.OffsetEnd-*res.OffsetStart)
		copy(chunkData, fileData[*res.OffsetStart:*res.OffsetEnd])
		fileChunk := base64.StdEncoding.EncodeToString(chunkData)
		if err != nil {
			return "", err
		}

		res, err = f.rpc.UploadData(f.wallet, oz.SequenceNumber, fileHash, fileChunk)
		if err != nil {
			if isDublErr(err.Error()) {
				return fileHash, nil
			}
			return "", err
		}
	}

	if res.Return != rpc_api.SUCCESS {
		if isDublErr(res.Return) {
			return fileHash, nil
		}
		return "", fmt.Errorf("failed sp upload data with error: %s", res.Return)
	}

	return fileHash, nil
}

func (f *Fetcher) Download(fileHash string) ([]byte, error) {
	oz, err := f.rpc.GetOzone(f.wallet)
	if err != nil {
		return nil, err
	}

	res, err := f.rpc.RequestDownload(f.wallet, oz.SequenceNumber, fileHash)
	fmt.Println("Fetcher Download RequestDownload res - err", res, err)
	if err != nil {
		return nil, err
	}

	var (
		fileSize uint64 = 0
		fileData        = make([]byte, 0)
	)

	// Handle result:1 sending the content
	for res.Return == rpc_api.DOWNLOAD_OK || res.Return == rpc_api.DL_OK_ASK_INFO {
		if res.Return == rpc_api.DL_OK_ASK_INFO {
			res, err = f.rpc.DownloadedFileInfo(f.wallet, res.ReqId, fileHash, fileSize)
		} else {
			start := *res.OffsetStart
			end := *res.OffsetEnd
			fileSize = fileSize + (end - start)
			decoded, _ := base64.StdEncoding.DecodeString(res.FileData)
			fileData = append(fileData, decoded...)
			res, err = f.rpc.DownloadData(f.wallet, res.ReqId, fileHash)
		}
		if err != nil {
			return nil, err
		}
	}
	if res.Return != rpc_api.SUCCESS {
		return nil, fmt.Errorf("failed sp download with error: %s", res.Return)
	}

	return fileData, nil
}
