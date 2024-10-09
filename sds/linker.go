package sds

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/ipfs/boxo/files"
	sdsprotos "github.com/ipfs/kubo/sds/protos"
)

func NewSdsFile(sdsLink *sdsprotos.SdsLinker) (files.Node, error) {
	b, err := proto.Marshal(sdsLink)
	if err != nil {
		return nil, err
	}
	rfc := files.NewBytesFile(b)
	return rfc, nil
}

func ParseLink(fileData []byte) (*sdsprotos.SdsLinker, error) {
	if len(fileData) == 0 {
		return nil, fmt.Errorf("empty file data")
	}
	link := &sdsprotos.SdsLinker{}
	err := proto.Unmarshal(fileData, link)
	if err != nil {
		return nil, err
	}

	return link, nil
}
