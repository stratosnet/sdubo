package sds

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/ipfs/boxo/files"
	cid "github.com/ipfs/go-cid"
	sdsprotos "github.com/ipfs/kubo/sds/protos"
)

func NewSdsFile(cid cid.Cid, fileHash string) (files.Node, error) {
	link := &sdsprotos.SdsLinker{
		OriginalCid: cid.String(),
		SdsFileHash: fileHash,
	}
	b, err := proto.Marshal(link)
	if err != nil {
		return nil, err
	}
	rfc := files.NewBytesFile(b)
	return rfc, nil
}

func ParseLink(data []byte) (cid.Cid, error) {
	if len(data) == 0 {
		return cid.Cid{}, fmt.Errorf("empty file data")
	}
	link := &sdsprotos.SdsLinker{}
	err := proto.Unmarshal(data, link)
	if err != nil {
		return cid.Cid{}, err
	}

	return cid.Parse(link.OriginalCid)
}
