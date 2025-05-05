package sg

import (
	"context"
	"fmt"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

type Reporter struct {
	rpcClient *Client
	mu        sync.RWMutex
}

// NOTE: Tmp storage, IMPLEMENT
var memStore map[string]ReportFileInfo

var repLogger = logging.Logger("sg/reporter")

func init() {
	memStore = make(map[string]ReportFileInfo)
}

func NewReporter(rpcClient *Client) *Reporter {
	return &Reporter{
		rpcClient: rpcClient,
	}
}

func (r *Reporter) Store(_ context.Context, key string, value ReportFileInfo) error {
	if err := value.Validate(); err != nil {
		return err
	}

	repLogger.Debugf("store cid '%s' for next report", key)

	r.mu.Lock()
	defer r.mu.Unlock()
	memStore[key] = value

	return nil
}

func (r *Reporter) Notify(ctx context.Context, key string) error {
	r.mu.RLock()
	data, ok := memStore[key]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("file '%s' does not exist, nothing to report", key)
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
