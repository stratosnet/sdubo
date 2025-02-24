package mfscl

import (
	"context"
	"fmt"
	"sync"

	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
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

func dedupKeys(keys []cid.Cid) []cid.Cid {
	set := cid.NewSet()
	for _, c := range keys {
		set.Add(c)
	}
	if set.Len() == len(keys) {
		return keys
	}
	return set.Keys()
}

// NewMFSClDagService constructs a new mfsClDagService (using the default implementation).
// Note that the default implementation is also an ipld.LinkGetter.
func NewMFSClDagService(cl *MFSCluster) *mfsClDagService {
	if cl == nil {
		panic("mfscl is nil")
	}
	return &mfsClDagService{
		cl:      cl,
		decoder: ipldLegacyDecoder,
	}
}

// mfsClmfsClDagService is an IPFS Merkle DAG service for remote cluster.
// - the root is virtual (like a forest)
// - stores nodes' data in a MFSCluster
type mfsClDagService struct {
	cl      *MFSCluster
	decoder *legacy.Decoder
}

func (n *mfsClDagService) makeKey(c cid.Cid) string {
	return n.cl.CreateKey("blocks", c.String())
}

// Add adds a node to the mfsClDagService, storing the block in the BlockService
func (n *mfsClDagService) Add(ctx context.Context, nd format.Node) error {
	fmt.Println("add cid", nd.Cid())
	fmt.Println("add data", nd.RawData())
	return n.cl.Put(ctx, n.makeKey(nd.Cid()), nd.RawData())
}

func (n *mfsClDagService) AddMany(ctx context.Context, nds []format.Node) error {
	// TODO: Add concurrent call
	for _, nd := range nds {
		if err := n.cl.Put(ctx, n.makeKey(nd.Cid()), nd.RawData()); err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves a node from the mfsClDagService, fetching the block in the BlockService
func (n *mfsClDagService) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pbData, err := n.cl.Get(ctx, n.cl.CreateKey("blocks", c.String()))
	if err != nil {
		return nil, err
	}

	fmt.Println("get cid", c)
	fmt.Println("get data", pbData)

	b := blocks.NewBlock(pbData)

	return n.decoder.DecodeNode(ctx, b)
}

// GetMany gets many nodes at once, batching the request if possible.
func (n *mfsClDagService) GetMany(ctx context.Context, keys []cid.Cid) <-chan *format.NodeOption {
	var wg sync.WaitGroup

	keys = dedupKeys(keys)
	out := make(chan *format.NodeOption, len(keys))

	go func() {
		defer close(out)

		for _, c := range keys {
			go func(c cid.Cid) {
				wg.Add(1)
				defer wg.Done()

				nd, err := n.Get(ctx, c)
				if err != nil {
					out <- &format.NodeOption{Err: err}
					return
				}

				out <- &format.NodeOption{Node: nd}

			}(c)
		}
		wg.Wait()
	}()
	return out
}

// GetLinks return the links for the node, the node doesn't necessarily have
// to exist locally.
func (n *mfsClDagService) GetLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	if c.Type() == cid.Raw {
		return nil, nil
	}
	node, err := n.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

func (n *mfsClDagService) Remove(ctx context.Context, c cid.Cid) error {
	fmt.Println("rm cid", c)
	return n.cl.Rm(ctx, n.makeKey(c))
}

// RemoveMany removes multiple nodes from the DAG. It will likely be faster than
// removing them individually.
//
// This operation is not atomic. If it returns an error, some nodes may or may
// not have been removed.
func (n *mfsClDagService) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	// TODO: Add concurrent call
	for _, c := range cids {
		if err := n.Remove(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ format.LinkGetter = &mfsClDagService{}
	_ format.NodeAdder  = &mfsClDagService{}
	_ format.NodeGetter = &mfsClDagService{}
	_ format.DAGService = &mfsClDagService{}
)
