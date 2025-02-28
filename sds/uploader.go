package sds

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange/offline"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
)

var uLog = logging.Logger("sds/uploader")

type uploadRequest struct {
	cid     cid.Cid
	privKey string
}

type Uploader struct {
	sync.Locker
	fetcher   *Fetcher
	dagParser *DagParser
	pinning   pin.Pinner
	dag       format.DAGService

	// timeouts
	statusCheckTimeout uint16

	reqCh chan *uploadRequest
}

func NewUploader(fetcher *Fetcher, pinning pin.Pinner, dag format.DAGService, bs blockstore.GCBlockstore) *Uploader {
	u := &Uploader{
		fetcher:            fetcher,
		dagParser:          NewDagParser(context.Background(), bs, pinning),
		pinning:            pinning,
		dag:                merkledag.NewDAGService(blockservice.New(bs, offline.Exchange(bs))),
		statusCheckTimeout: 30 * 60,
		reqCh:              make(chan *uploadRequest, 100),
	}

	go u.initRefetch()
	go u.uploadConsumer()

	return u
}

func (u *Uploader) initRefetch() error {
	ctx := context.Background()

	for streamedCid := range u.pinning.RecursiveKeys(ctx, false) {
		uLog.Debugf(
			"Got streamed pin key with CID: %s from recursive keys to proceed (err: %v)",
			streamedCid.Pin.Key, streamedCid.Err,
		)
		if streamedCid.Err != nil {
			return streamedCid.Err
		}
		nd, err := u.dag.Get(ctx, streamedCid.Pin.Key)
		uLog.Debugf("Dag obtained for CID: %s with err: %v", streamedCid.Pin.Key, err)
		if err != nil {
			return err
		}

		originCid, err := ParseLink(nd.RawData())
		uLog.Debugf("Origin CID: %s from original CID: %s parsed with err: %v", originCid, streamedCid.Pin.Key, err)
		if err != nil {
			return err
		}

		uLog.Debugf("Adding CID %s to a queue from refetch", originCid)
		// we do not know pk at this point
		u.reqCh <- &uploadRequest{
			cid:     originCid,
			privKey: "",
		}
	}

	return nil
}

func (u *Uploader) upload(c cid.Cid, privKey string) error {
	f, err := u.dagParser.Export(c)
	uLog.Debugf("Export finished for CID: %s", c)
	if err != nil {
		return err
	}

	fileData, err := io.ReadAll(f)
	uLog.Debugf("File read for CID: %s", c)
	if err != nil {
		return err
	}

	fileHash, err := u.fetcher.Upload(privKey, fileData)
	uLog.Debugf("Upload done for CID: %s with err: %s", c, err)
	if err != nil {
		return err
	}

	_, err = u.fetcher.CheckStatus(privKey, fileHash, uint16(u.statusCheckTimeout))
	uLog.Debugf("Status check done for CID: %s with err: %s", c, err)
	if err != nil {
		return err
	}

	_, err = u.fetcher.CreateShareLink(privKey, fileHash, c.String())
	uLog.Debugf("Share link creation done for CID: %s with err: %s", c, err)
	if err != nil {
		return err
	}

	return nil
}

func (u *Uploader) uploadConsumer() {
	resCh := make(chan struct {
		cid cid.Cid
		err error
	})
	for {
		select {
		case ui := <-u.reqCh:
			go func(info *uploadRequest) {
				operation := func() error {
					uLog.Debugf("Execute upload operation in retry for CID: %s", info.cid)
					return u.upload(info.cid, info.privKey)
				}
				bo := backoff.NewExponentialBackOff()
				bo.InitialInterval = 15 * time.Second
				bo.Multiplier = 3
				bo.MaxInterval = 30 * time.Minute
				bo.MaxElapsedTime = 0 // never stop

				err := backoff.Retry(operation, bo)
				resCh <- struct {
					cid cid.Cid
					err error
				}{cid: info.cid, err: err}
			}(ui)
		case res := <-resCh:
			if res.err != nil {
				logger.Warningf("Upload failed for CID %s: %v", res.cid, res.err)
			} else {
				logger.Debugf("Upload compeleted for CID %s", res.cid)
			}
		}
	}
}

func (u *Uploader) AddToQueue(c cid.Cid, privKey string) error {
	uLog.Debugf("Adding CID %s to a queue", c)
	u.reqCh <- &uploadRequest{
		cid:     c,
		privKey: privKey,
	}
	return nil
}
