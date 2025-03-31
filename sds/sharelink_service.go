package sds

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

type ShareLinkData struct {
	Cid        cid.Cid   `json:"cid"`
	FileHash   string    `json:"fileHash"`
	PrivateKey string    `json:"privateKey"`
	ExpiredAt  time.Time `json:"expiredAt"`
}

func (d *ShareLinkData) ToBytes() ([]byte, error) {
	return json.Marshal(d)
}

func (d *ShareLinkData) FromBytes(data []byte) error {
	return json.Unmarshal(data, d)
}

type ShareLinkService struct {
	ds           ds.Datastore
	fetcher      *Fetcher
	prefix       string
	ttl          int
	tickInterval int

	mu sync.Mutex
}

var (
	slsservice *ShareLinkService
	once       sync.Once
)

func NewShareLinkService(ds ds.Datastore, fetcher *Fetcher) *ShareLinkService {
	once.Do(func() {
		if ds == nil {
			panic("datastore is nil")
		}
		slsservice = &ShareLinkService{
			ds:           ds,
			fetcher:      fetcher,
			prefix:       "/slserv",
			ttl:          3 * 24 * 60 * 60, // 3 days
			tickInterval: 60,               // 1 min
		}
		go slsservice.runCreateShareLinks()
	})
	return slsservice
}

func (s *ShareLinkService) createKey(c cid.Cid) ds.Key {
	return ds.NewKey(fmt.Sprintf("%s/%s", s.prefix, c.String()))
}

func (s *ShareLinkService) runCreateShareLinks() {
	ticker := time.NewTicker(time.Duration(s.tickInterval) * time.Second)

	defer ticker.Stop()

	for range ticker.C {
		entries, err := s.GetEntries(context.TODO())
		if err != nil {
			logger.Warnf("Failed to query share link service data: %v", err)
			continue
		}

		logger.Debugf("Got entries length: %d for share links creation", len(entries))

		for _, entry := range entries {
			sdata := &ShareLinkData{}
			err := sdata.FromBytes(entry.Value)
			if err != nil {
				logger.Warnf("Failed to get bytes for key '%s', details: %v", entry.Key, err)
				continue
			}
			logger.Debugf("Got entry for share link: %s - %s", sdata.Cid, sdata.FileHash)

			if !sdata.ExpiredAt.IsZero() && time.Now().After(sdata.ExpiredAt) {
				logger.Debugf("Share link expired for CID: %s, expired at: %v", sdata.Cid, sdata.ExpiredAt)
				if err := s.Remove(context.TODO(), sdata.Cid); err != nil {
					logger.Warnf("Failed to remove expired share link '%s': %v", sdata.Cid, err)
				}
				continue
			}

			found, _ := s.fetcher.CheckStatus(sdata.PrivateKey, sdata.FileHash, 0)
			if found {
				logger.Debugf("Share link found '%s', skip", sdata.Cid)
				continue
			}

			if _, err := s.fetcher.CreateShareLink(sdata.PrivateKey, sdata.FileHash, sdata.Cid.String()); err != nil {
				logger.Warnf("failed to create share link details: %v", err)
				continue
			}

			if err := s.Remove(context.TODO(), sdata.Cid); err != nil {
				logger.Warnf("Failed to remove share link in store '%s' details: %v", sdata.Cid, err)
			}
		}
	}
}

func (s *ShareLinkService) GetEntries(ctx context.Context) ([]query.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	results, err := s.ds.Query(context.TODO(), query.Query{
		Prefix: s.prefix,
	})
	if err != nil {
		return nil, err
	}
	return results.Rest()
}

func (s *ShareLinkService) Add(ctx context.Context, c cid.Cid, fileHash, privateKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expiredAt time.Time
	if s.ttl > 0 {
		expiredAt = time.Now().Add(time.Duration(s.ttl) * time.Second)
	}

	data := &ShareLinkData{
		Cid:        c,
		FileHash:   fileHash,
		PrivateKey: privateKey,
		ExpiredAt:  expiredAt,
	}
	res, err := data.ToBytes()
	if err != nil {
		return err
	}
	return s.ds.Put(ctx, s.createKey(c), res)
}

func (s *ShareLinkService) Remove(ctx context.Context, c cid.Cid) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ds.Delete(ctx, s.createKey(c))
}
