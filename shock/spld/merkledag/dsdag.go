package merkledag

import (
	"context"
	"sync"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	legacy "github.com/ipfs/go-ipld-legacy"
)

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

type createDagKey func(c cid.Cid) ds.Key

// NewDsDagService constructs a new dsDagService (using the default implementation).
// Note that the default implementation is also an ipld.LinkGetter.
func NewDsDagService(ds ds.Datastore, keyMaker createDagKey) format.DAGService {
	if ds == nil {
		panic("datastore is nil")
	}
	return &dsDagService{
		ds:       ds,
		keyMaker: keyMaker,
		decoder:  ipldLegacyDecoder,
	}
}

// dsDagService is an IPFS Merkle DAG service for remote or local use.
// - the root is virtual (like a forest)
// - stores nodes' data in a some db
type dsDagService struct {
	ds       ds.Datastore
	keyMaker createDagKey
	decoder  *legacy.Decoder
}

// Add adds a node to the dsDagService, storing the block in the BlockService
func (n *dsDagService) Add(ctx context.Context, nd format.Node) error {
	log.Debugf("ds dag service add cid: %s", nd.Cid())
	log.Debugf("ds dag service add raw data: %b", nd.RawData())
	return n.ds.Put(ctx, n.keyMaker(nd.Cid()), nd.RawData())
}

func (n *dsDagService) AddMany(ctx context.Context, nds []format.Node) error {
	// TODO: Add concurrent call
	for _, nd := range nds {
		if err := n.ds.Put(ctx, n.keyMaker(nd.Cid()), nd.RawData()); err != nil {
			return err
		}
	}
	return nil
}

// Get retrieves a node from the dsDagService, fetching the block in the BlockService
func (n *dsDagService) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pbData, err := n.ds.Get(ctx, n.keyMaker(c))
	if err != nil {
		return nil, err
	}

	log.Debugf("ds dag service get cid: %s", c)
	log.Debugf("ds dag service get raw data: %b", pbData)

	b := blocks.NewBlock(pbData)

	return n.decoder.DecodeNode(ctx, b)
}

// GetMany gets many nodes at once, batching the request if possible.
func (n *dsDagService) GetMany(ctx context.Context, keys []cid.Cid) <-chan *format.NodeOption {
	keys = dedupKeys(keys)
	out := make(chan *format.NodeOption, len(keys))

	var wg sync.WaitGroup
	wg.Add(len(keys))

	for _, c := range keys {
		go func(c cid.Cid) {
			defer wg.Done()
			nd, err := n.Get(ctx, c)
			if err != nil {
				out <- &format.NodeOption{Err: err}
				return
			}

			out <- &format.NodeOption{Node: nd}

		}(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// GetLinks return the links for the node, the node doesn't necessarily have
// to exist locally.
func (n *dsDagService) GetLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	if c.Type() == cid.Raw {
		return nil, nil
	}
	node, err := n.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

func (n *dsDagService) Remove(ctx context.Context, c cid.Cid) error {
	log.Debugf("ds dag service remove cid: %s", c)
	return n.ds.Delete(ctx, n.keyMaker(c))
}

// RemoveMany removes multiple nodes from the DAG. It will likely be faster than
// removing them individually.
//
// This operation is not atomic. If it returns an error, some nodes may or may
// not have been removed.
func (n *dsDagService) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	// TODO: Add concurrent call
	for _, c := range cids {
		if err := n.Remove(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ format.LinkGetter = &dsDagService{}
	_ format.NodeAdder  = &dsDagService{}
	_ format.NodeGetter = &dsDagService{}
	_ format.DAGService = &dsDagService{}
)
