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
	ProjectID   int     `json:"project_id"`
	IPFSCid     cid.Cid `json:"ipfs_cid"`
	SdsCid      cid.Cid `json:"sds_cid"`
	SdsFileHash string  `json:"sds_file_hash"`
	FileSize    uint64  `json:"file_size"`
}

func (r *ReportFileInfo) Validate() error {
	if r.ProjectID == 0 {
		return fmt.Errorf("project_id not set")
	}
	if len(r.IPFSCid.Bytes()) == 0 {
		return fmt.Errorf("ipfs_cid not set")
	}
	if len(r.SdsCid.Bytes()) == 0 {
		return fmt.Errorf("sds_cid not set")
	}
	if r.SdsFileHash == "" {
		return fmt.Errorf("sds_file_hash not set")
	}
	if r.FileSize == 0 {
		return fmt.Errorf("file_size not set")
	}
	return nil
}
