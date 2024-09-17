package sds

import (
	"encoding/json"
	"fmt"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
)

type SdsLinker struct {
	OriginalCID string `json:"originalCID,omitempty"`
	SdsFileHash string `json:"sdsFileHash,omitempty"`
}

func NewSdsFile(cidLink path.ImmutablePath, sdsFileHash string) (files.Node, error) {
	link := &SdsLinker{
		OriginalCID: cidLink.RootCid().String(),
		SdsFileHash: sdsFileHash,
	}
	b, err := json.Marshal(link)
	if err != nil {
		return nil, err
	}
	rfc := files.NewBytesFile(b)
	// mapFile := files.NewMapDirectory(map[string]files.Node{
	// 	"sdsMapper": rfc,
	// })
	return rfc, nil
}

func ParseLink(fileData []byte) (string, error) {
	if len(fileData) == 0 {
		return "", fmt.Errorf("empty file data")
	}
	link := &SdsLinker{}
	err := json.Unmarshal(fileData, link)
	if err != nil {
		return "", err
	}

	return link.SdsFileHash, nil
}
