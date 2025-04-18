package merkledag

import (
	"context"
	"sync"

	cid "github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
)

// NewMultiDagService constructs a new multiDagService (using the default implementation).
func NewMultiDagService(readers []format.NodeGetter, writers []format.NodeAdder, removers []format.DAGService) format.DAGService {
	if len(readers) == 0 {
		panic("reader dags are empty")
	}
	if len(writers) == 0 {
		panic("reader dags are empty")
	}
	if len(removers) == 0 {
		panic("reader dags are empty")
	}
	return &multiDagService{
		readers:  readers,
		writers:  writers,
		removers: removers,
	}
}

// multiDagService is an IPFS Merkle DAG service with multiple r w r services
type multiDagService struct {
	readers  []format.NodeGetter
	writers  []format.NodeAdder
	removers []format.DAGService
}

// Add adds a node to the dsDagService, storing the block in the BlockService
func (n *multiDagService) Add(ctx context.Context, nd format.Node) error {
	log.Debugf("multi dag service add cid: %s", nd.Cid())
	for _, r := range n.writers {
		if err := r.Add(ctx, nd); err != nil {
			return err
		}
	}
	return nil
}

func (n *multiDagService) AddMany(ctx context.Context, nds []format.Node) error {
	// TODO: Add concurrent call
	for _, nd := range nds {
		for _, r := range n.writers {
			if err := r.Add(ctx, nd); err != nil {
				return err
			}
		}
	}
	return nil
}

// Get retrieves a node from the multiDagService, fetching the block in the BlockService
func (n *multiDagService) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Debugf("multi dag service get cid: %s", c)

	var (
		nd  format.Node
		err error
	)

	for _, r := range n.readers {
		nd, err = r.Get(ctx, c)
		if err == nil {
			log.Debugf("multi dag service get raw data for key: %s", c)
			break
		}
	}

	// in case of node found on another iteration, we should clear err
	if nd != nil {
		err = nil
	}

	return nd, err
}

// GetMany gets many nodes at once, batching the request if possible.
func (n *multiDagService) GetMany(ctx context.Context, keys []cid.Cid) <-chan *format.NodeOption {
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
func (n *multiDagService) GetLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	if c.Type() == cid.Raw {
		return nil, nil
	}
	node, err := n.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

func (n *multiDagService) Remove(ctx context.Context, c cid.Cid) error {
	log.Debugf("multi dag service remove cid: %s", c)
	for _, r := range n.removers {
		if err := r.Remove(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

// RemoveMany removes multiple nodes from the DAG. It will likely be faster than
// removing them individually.
//
// This operation is not atomic. If it returns an error, some nodes may or may
// not have been removed.
func (n *multiDagService) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	// TODO: Add concurrent call
	for _, c := range cids {
		if err := n.Remove(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

var (
	_ format.LinkGetter = &multiDagService{}
	_ format.NodeAdder  = &multiDagService{}
	_ format.NodeGetter = &multiDagService{}
	_ format.DAGService = &multiDagService{}
)
