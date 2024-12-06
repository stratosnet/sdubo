package sds

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/ipld/unixfs"
	cid "github.com/ipfs/go-cid"
	sdsprotos "github.com/ipfs/kubo/sds/protos"
	mc "github.com/multiformats/go-multicodec"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
)

type vData struct {
	link   *sdsprotos.SdsLinker
	result string
}

func TestData_CidV0(t *testing.T) {
	tests := []*vData{
		{
			link: &sdsprotos.SdsLinker{
				OriginalCid: "QmdNvdx1xS4GM8iz6VAKBjCWxN9VfPRDawhH8AvQ7nsMoc",
				SdsFileHash: "v05j1m542h591p6r17kg0om0sud987gclm1e0akg",
			},
			result: "QmNQyVytMFehieafSxu8rgxzHJuhRqVcpcTmA9BiTAq8BT",
		},
	}

	for _, t_ := range tests {
		fileData, err := proto.Marshal(t_.link)
		if err != nil {
			panic(err)
		}

		pb := merkledag.NodeWithData(unixfs.FilePBData(fileData, uint64(len(fileData))))

		assert.Equal(t, t_.result, pb.Cid().String())
	}
}

func TestData_CidV1(t *testing.T) {
	var cidPrefix cid.Prefix

	// Baseline is CIDv1 raw sha2-255-32 (can be tweaked later via opts)
	cidPrefix.Version = 1
	cidPrefix.Codec = uint64(mc.Raw)
	cidPrefix.MhType = mh.SHA2_256
	cidPrefix.MhLength = -1 // -1 means len is to be calculated during mh.Sum()

	tests := []*vData{
		{
			link: &sdsprotos.SdsLinker{
				OriginalCid: "bafkreih35zewnawrf4fnazqfaljwbhqh4f27pnmigfdgolfwqzt33a2guu",
				SdsFileHash: "v05j1m500rni6ocqqp4lrl6r8ksfeheedc2j6sig",
			},
			result: "bafkreifvk6nj4bok7xiylptmjmvosbz34stzerzgpo7vuzhplfqthacrim",
		},
	}

	for _, t_ := range tests {
		fileData, err := proto.Marshal(t_.link)
		if err != nil {
			panic(err)
		}

		cid_, err := cidPrefix.Sum(fileData)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, t_.result, cid_.String())
	}
}
