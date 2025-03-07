package merkledag

import (
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	cid "github.com/ipfs/go-cid"
	legacy "github.com/ipfs/go-ipld-legacy"
	dagpb "github.com/ipld/go-codec-dagpb"

	// blank import is used to register the IPLD raw codec
	_ "github.com/ipld/go-ipld-prime/codec/raw"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
)

var ipldLegacyDecoder *legacy.Decoder

// TODO: Don't require global registries
func init() {
	d := legacy.NewDecoder()
	d.RegisterCodec(cid.DagProtobuf, dagpb.Type.PBNode, merkledag.ProtoNodeConverter)
	d.RegisterCodec(cid.Raw, basicnode.Prototype.Bytes, merkledag.RawNodeConverter)
	ipldLegacyDecoder = d
}
