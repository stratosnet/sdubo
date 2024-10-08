package sds

import (
	"encoding/json"
	"fmt"

	"github.com/ipfs/boxo/files"
)

type SdsLinker struct {
	OriginalCID string `json:"originalCID,omitempty"`
	SdsFileHash string `json:"sdsFileHash,omitempty"`
}

func NewSdsFile(sdsLink *SdsLinker) (files.Node, error) {
	link := &SdsLinker{
		OriginalCID: sdsLink.OriginalCID,
		SdsFileHash: sdsLink.SdsFileHash,
	}
	b, err := json.Marshal(link)
	if err != nil {
		return nil, err
	}
	rfc := files.NewBytesFile(b)
	return rfc, nil
}

func ParseLink(fileData []byte) (*SdsLinker, error) {
	if len(fileData) == 0 {
		return nil, fmt.Errorf("empty file data")
	}
	link := &SdsLinker{}
	err := json.Unmarshal(fileData, link)
	if err != nil {
		return nil, err
	}

	return link, nil
}
