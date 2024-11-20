package sds

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipfs/kubo/config"
	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
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
	wallet         *SdsWallet
	rpc            *Rpc
	q              *fqueue
	execRetryCount uint8
}

func NewFetcher(cfg *config.Sds, noRetry bool) (*Fetcher, error) {
	wallet, err := NewSdsWallet(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	rpc, err := NewRpc(cfg.RpcURL)
	if err != nil {
		return nil, err
	}

	f := &Fetcher{
		cfg:    cfg,
		wallet: wallet,
		rpc:    rpc,
		q:      newFqueue(30, 100),
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

func (f *Fetcher) retryFetchPoll() {
	defer f.q.ticker.Stop()

	for {
		select {
		case <-f.q.ticker.C:
			f.retry()
		}
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

func (f *Fetcher) download(fileHash string, downloadCallback func() (*rpc_api.Result, error)) ([]byte, error) {
	var (
		fileSize uint64 = 0
	)

	res, err := downloadCallback()
	if err != nil {
		return nil, err
	}

	if fileHash == "" {
		fileHash = res.FileHash
	}

	filePath := filepath.Join(f.cfg.CacheFolder, fileHash)

	fileData, err := readFile(filePath)
	if err != nil {
		return nil, err
	}
	if fileData != nil {
		return fileData, err
	}

	if fileData == nil {
		fileData = make([]byte, 0)
	}

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

	if err = writeOnly(filePath, fileData[:]); err != nil {
		return nil, err
	}

	return fileData, nil
}

func (f *Fetcher) Download(fileHash string) ([]byte, error) {
	callback := func() (*rpc_api.Result, error) {
		oz, err := f.rpc.GetOzone(f.wallet)
		if err != nil {
			return nil, err
		}
		res, err := f.rpc.RequestDownload(f.wallet, oz.SequenceNumber, fileHash)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	return f.download(fileHash, callback)
}

func (f *Fetcher) DownloadFromShare(shareLink string) ([]byte, error) {
	callback := func() (*rpc_api.Result, error) {
		oz, err := f.rpc.GetOzone(f.wallet)
		if err != nil {
			return nil, err
		}
		res, err := f.rpc.GetShared(f.wallet, oz.SequenceNumber, shareLink)
		fmt.Println("Fetcher Download DownloadFromShare res - err", res, err)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	return f.download("", callback)
}

func (f *Fetcher) CreateShareLink(fileHash, cid string) (bool, error) {
	fn := func() error {
		res, err := f.rpc.RequestShare(f.wallet, fileHash, &cid)
		fmt.Println("Fetcher CreateShareLink RequestShare res - err", res, err)
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
