package sg

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
)

type Reporter struct {
	rpcClient *Client
	ds        datastore.Datastore
}

var repLogger = logging.Logger("sg/reporter")

func NewReporter(rpcClient *Client, ds datastore.Datastore) *Reporter {
	return &Reporter{
		rpcClient: rpcClient,
		ds:        ds,
	}
}

func (r *Reporter) makeKey(key string) datastore.Key {
	return datastore.NewKey(fmt.Sprintf("/reporter/%s", key))
}

func (r *Reporter) Store(ctx context.Context, key string, value ReportFileInfo) error {
	if err := value.Validate(); err != nil {
		return err
	}

	repLogger.Debugf("store cid '%s' for next report", key)

	data, err := json.Marshal(&value)
	if err != nil {
		return err
	}

	if err := r.ds.Put(ctx, r.makeKey(key), data); err != nil {
		return err
	}

	return nil
}

func (r *Reporter) Notify(ctx context.Context, key string) error {
	value, err := r.ds.Get(ctx, r.makeKey(key))
	if err != nil {
		return err
	}

	data := &ReportFileInfo{}
	if err := json.Unmarshal(value, data); err != nil {
		return err
	}

	req := &TrafficRequest{
		ProjectID: data.ProjectID,
		Traffic:   data.FileSize,
		Time:      time.Now(),
	}

	repLogger.Debugf("notify sg with cid '%s'", key)

	if err := r.rpcClient.UpdateTraffic(ctx, req); err != nil {
		return err
	}

	return nil
}
