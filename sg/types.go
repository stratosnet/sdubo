package sg

import (
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
)

type IParams interface {
	ToParams() map[string]any
}

type Request struct {
	Path    string
	Method  string
	Json    []byte
	Params  IParams
	Headers map[string]string
}

type TrafficRequest struct {
	ProjectID int       `json:"project_id"`
	Traffic   uint64    `json:"traffic"`
	Time      time.Time `json:"time"`
}

type ReportFileInfo struct {
	ProjectID   int
	Cid         cid.Cid
	SdsFileHash string
	FileSize    uint64
}

func (r *ReportFileInfo) Validate() error {
	if r.ProjectID == 0 {
		return fmt.Errorf("project id not set")
	}
	if len(r.Cid.Bytes()) == 0 {
		return fmt.Errorf("cid not set")
	}
	if r.SdsFileHash == "" {
		return fmt.Errorf("sds file hash size not set")
	}
	if r.FileSize == 0 {
		return fmt.Errorf("file size not set")
	}
	return nil
}
