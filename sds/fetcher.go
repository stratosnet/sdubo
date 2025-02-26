package sds

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/misc/sutil"
	fwtypes "github.com/stratosnet/sds/framework/types"
	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
	"github.com/stratosnet/sds/sds-msg/protos"
)

type execInfo struct {
	name       string
	fn         func() error
	retryCount uint8
}

type fqueue struct {
	ch     chan *execInfo // Channel to hold the queue elements in FIFO order
	rCh    chan *execInfo // Channel to hold the queue elements in FIFO order to retry
	ticker *time.Ticker   // Ticker for re-try mechanics
}

// newFqueue creates a new instance of fqueue with a specific size
func newFqueue(pollRetryTime, size int) *fqueue {
	return &fqueue{
		ch:     make(chan *execInfo, size),
		rCh:    make(chan *execInfo, size),
		ticker: time.NewTicker(time.Duration(pollRetryTime) * time.Second),
	}
}

// RetryQueue returns the channel of the fqueue to retry
func (q *fqueue) RetryQueue() chan *execInfo {
	return q.rCh
}

// Queue returns the channel of the fqueue
func (q *fqueue) Queue() chan *execInfo {
	return q.ch
}

type Fetcher struct {
	cfg            *config.Sds
	rpc            *Rpc
	q              *fqueue
	execRetryCount uint8
	pollTimeout    uint8
}

func NewFetcher(cfg *config.Sds, noRetry bool) (*Fetcher, error) {
	addr, err := sutil.ParseHTTPAddress(cfg.RPC)
	if err != nil {
		return nil, err
	}

	rpc, err := NewRpc(addr)
	if err != nil {
		return nil, err
	}

	f := &Fetcher{
		cfg:         cfg,
		rpc:         rpc,
		q:           newFqueue(30, 100),
		pollTimeout: 15,
	}

	if !noRetry {
		f.execRetryCount = 5
		go f.fetchPoll()
		go f.retryFetchPoll()
	}

	return f, nil
}

func isDublErr(ret string) bool {
	// this is sp error, means that file already exist and uploaded, so we could just link
	return strings.Contains(ret, "Same file with the name")
}

func isMethodNotFound(ret string) bool {
	return strings.Contains(ret, "does not exist/is not available")
}

func (f *Fetcher) retryFetchPoll() {
	defer f.q.ticker.Stop()

	for range f.q.ticker.C {
		f.retry()
	}
}

func (f *Fetcher) retry() {
	for {
		select {
		case item := <-f.q.RetryQueue():
			if item.retryCount > 0 {
				item.retryCount -= 1
			}
			f.q.Queue() <- item
		default:
			return
		}
	}
}

func (f *Fetcher) fetchPoll() {
	for item := range f.q.Queue() {
		if err := f.execute(item); err != nil {
			if item.retryCount != 0 {
				f.q.RetryQueue() <- item
			}
		}
	}
}

func (f *Fetcher) execute(ei *execInfo) error {
	if err := ei.fn(); err != nil {
		// TODO: Maybe handle only -5 (timeout)?
		return err
	}
	return nil
}

func (f *Fetcher) getWallet(privKey string) (*SdsWallet, error) {
	// In case empty wallet
	if privKey == "" {
		privKey = f.cfg.PrivateKey
	}
	wallet, err := NewSdsWallet(privKey)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func (f *Fetcher) CheckStatus(privKey string, fileHash string, pollInterval uint16) (bool, error) {
	wallet, err := f.getWallet(privKey)
	if err != nil {
		return false, err
	}

	done := make(chan struct{})
	errChan := make(chan error)
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	go func() {
		pollFileStatus := func() {
			fsr, err := f.rpc.GetFileStatus(wallet, fileHash)
			if err != nil {
				errChan <- err
			}
			if fsr.FileUploadState == protos.FileUploadState_FINISHED {
				done <- struct{}{}
			}
		}

		pollFileStatus()
		for range ticker.C {
			pollFileStatus()
		}
	}()

	select {
	case <-done:
		return true, nil
	case err := <-errChan:
		return false, err
	case <-time.After(time.Duration(f.pollTimeout) * time.Second):
		return false, fmt.Errorf("file upload timed out")
	}
}

func (f *Fetcher) Upload(privKey string, fileData []byte) (string, error) {
	fileHash := sutil.CreateFileHash(fileData)

	wallet, err := f.getWallet(privKey)
	if err != nil {
		return "", err
	}

	oz, err := f.rpc.GetOzone(wallet)
	if err != nil {
		return "", err
	}

	// TODO: How to get file name?
	fileName, err := sutil.RandomFileName(16, "txt")
	if err != nil {
		return "", err
	}

	res, err := f.rpc.RequestUpload(wallet, oz.SequenceNumber, fileName, fileHash, len(fileData))
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

		res, err = f.rpc.UploadData(wallet, oz.SequenceNumber, fileHash, fileChunk)
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

	oz, err = f.rpc.GetOzone(wallet)
	if err != nil {
		return "", err
	}

	resp, err := f.rpc.UploadSign(wallet, oz.SequenceNumber, fileHash)
	logger.Debugf("rpc.UploadSign resp: %+v, err: %v", resp, err)
	if err != nil {
		if isDublErr(err.Error()) || isMethodNotFound(err.Error()) {
			return fileHash, nil
		}
		return "", err
	}

	if isDublErr(resp.Return) {
		return fileHash, nil
	}

	return fileHash, nil
}

func (f *Fetcher) download(wallet *SdsWallet, fileHash string, downloadCallback func(sequenceNumber string) (*rpc_api.Result, error)) ([]byte, error) {
	var (
		fileSize uint64 = 0
	)

	oz, err := f.rpc.GetOzone(wallet)
	if err != nil {
		return nil, err
	}

	res, err := downloadCallback(oz.SequenceNumber)
	if err != nil {
		return nil, err
	}

	if fileHash == "" {
		fileHash = res.FileHash
	}

	reqId := res.ReqId

	fileData := make([]byte, res.FileSize)

	// Handle result:1 sending the content
	for res.Return == rpc_api.DOWNLOAD_OK || res.Return == rpc_api.DL_OK_ASK_INFO {
		if res.Return == rpc_api.DL_OK_ASK_INFO {
			res, err = f.rpc.DownloadedFileInfo(wallet, reqId, fileHash, fileSize)
		} else {
			start := *res.OffsetStart
			end := *res.OffsetEnd

			fileSize = fileSize + (end - start)
			decoded, _ := base64.StdEncoding.DecodeString(res.FileData)

			if len(decoded) != int(end-start) {
				return nil, fmt.Errorf("wrong size betwee decoded (%d) and offset (%d)", len(decoded), end-start)
			}
			copy(fileData[start:end], decoded)

			res, err = f.rpc.DownloadData(wallet, reqId, fileHash)
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

func (f *Fetcher) Download(privKey, fileHash string) ([]byte, error) {
	wallet, err := f.getWallet(privKey)
	if err != nil {
		return nil, err
	}

	callback := func(sequenceNumber string) (*rpc_api.Result, error) {
		res, err := f.rpc.RequestDownload(wallet, sequenceNumber, fileHash)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	return f.download(wallet, fileHash, callback)
}

func (f *Fetcher) DownloadFromShare(privKey, shareLink string) ([]byte, error) {
	wallet, err := f.getWallet(privKey)
	if err != nil {
		return nil, err
	}

	parsedLink, err := fwtypes.ParseShareLink(shareLink)
	if err != nil {
		return nil, err
	}

	callback := func(sequenceNumber string) (*rpc_api.Result, error) {
		res, err := f.rpc.GetShared(wallet, sequenceNumber, parsedLink)
		logger.Debugf("Fetcher Download DownloadFromShare res - err: %s", err)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	return f.download(wallet, "", callback)
}

func (f *Fetcher) CreateShareLink(privKey, fileHash, cid string) (bool, error) {
	wallet, err := f.getWallet(privKey)
	if err != nil {
		return false, err
	}

	fn := func() error {
		res, err := f.rpc.RequestShare(wallet, fileHash, &cid)
		logger.Debugf("Fetcher CreateShareLink RequestShare res - err: %s", err)
		if err != nil {
			return err
		}

		if res.Return != rpc_api.SUCCESS {
			return fmt.Errorf("share link creation failed, status code: %s", res.Return)
		}

		return nil
	}

	if f.execRetryCount > 0 {
		f.q.Queue() <- &execInfo{
			name:       "CreateShareLink",
			fn:         fn,
			retryCount: f.execRetryCount,
		}
	} else {
		if err := fn(); err != nil {
			return false, err
		}
	}

	return true, nil
}
