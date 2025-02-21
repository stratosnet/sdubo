package sds

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/ipfs/boxo/files"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	unixfs "github.com/ipfs/boxo/ipld/unixfs"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	sdsprotos "github.com/ipfs/kubo/sds/protos"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
)

func NewSdsMerkleDag(cid_ cid.Cid, fileHash string) (*blocks.BasicBlock, error) {
	var cidPrefix cid.Prefix

	// Baseline is CIDv1 raw sha2-255-32 (can be tweaked later via opts)
	cidPrefix.Version = 1
	cidPrefix.Codec = uint64(mc.Raw)
	cidPrefix.MhType = mh.SHA2_256
	cidPrefix.MhLength = -1 // -1 means len is to be calculated during mh.Sum()

	link := &sdsprotos.SdsLinker{
		OriginalCid: cid_.String(),
		SdsFileHash: fileHash,
	}
	fileData, err := proto.Marshal(link)
	if err != nil {
		return nil, err
	}

	if cid_.Version() == 0 {
		pb := merkledag.NodeWithData(unixfs.FilePBData(fileData, uint64(len(fileData))))
		return blocks.NewBlock(pb.RawData()), nil
	} else {
		bcid_, err := cidPrefix.Sum(fileData)
		if err != nil {
			return nil, err
		}
		b, err := blocks.NewBlockWithCid(fileData, bcid_)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
}

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
