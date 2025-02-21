package sds

import (
	"testing"

	cid "github.com/ipfs/go-cid"
	sdsprotos "github.com/ipfs/kubo/sds/protos"
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
		{
			link: &sdsprotos.SdsLinker{
				OriginalCid: "QmP1RWWCx1Ytz8QvqnXz7wTyDjtaN1BFJ1SXRt5JDsa4nR",
				SdsFileHash: "v05j1m569j3lacgj1tcocjc0lotv36si70s9vu78",
			},
			result: "QmaG5yjSi1FoxrXdXdpa9qfft6ckvTNEh4rN5R1JNxKYyv",
		},
	}

	for _, t_ := range tests {
		pb, err := NewSdsMerkleDag(cid.MustParse(t_.link.OriginalCid), t_.link.SdsFileHash)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, t_.result, pb.Cid().String())
	}
}

func TestData_CidV1(t *testing.T) {
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
		pb, err := NewSdsMerkleDag(cid.MustParse(t_.link.OriginalCid), t_.link.SdsFileHash)
		if err != nil {
			panic(err)
		}

		assert.Equal(t, t_.result, pb.Cid().String())
	}
}
