package sds

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/files"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/path"
	pin "github.com/ipfs/boxo/pinning/pinner"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	ipldlegacy "github.com/ipfs/go-ipld-legacy"
	"github.com/ipfs/kubo/misc/sutil"
	gocar "github.com/ipld/go-car"
	gocarv2 "github.com/ipld/go-car/v2"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
)

const (
	SoftBlockLimit = 1024 * 1024
)

type CutDagService interface {
	Add(context.Context, ipld.Node) error
	AddMany(context.Context, []ipld.Node) error
	Get(context.Context, cid.Cid) (ipld.Node, error)
}

type DagParser struct {
	ctx context.Context
	dag CutDagService
	bs  blockstore.GCBlockstore
	pin pin.Pinner
}

func NewDagParser(ctx context.Context, bs blockstore.GCBlockstore, pin pin.Pinner) *DagParser {
	return &DagParser{
		ctx: ctx,
		bs:  bs,
		pin: pin,
		dag: merkledag.NewDAGService(blockservice.New(bs, offline.Exchange(bs))),
	}
}

func (dp *DagParser) Get(_ context.Context, c cid.Cid) (blocks.Block, error) {
	return dp.dag.Get(dp.ctx, c)
}

func (dp *DagParser) Exists(c cid.Cid) bool {
	if blk, err := dp.Get(context.TODO(), c); err == nil && blk != nil {
		logger.Debugf("Import block: %s found, details: %+v", c, blk)
		return true
	}
	return false
}

func (dp *DagParser) Import(file files.File, doPinRoots bool) (path.Path, error) {
	logger.Debugf("Starting to import file with pin: %t", doPinRoots)
	blockDecoder := ipldlegacy.NewDecoder()

	// grab a pinlock ( which doubles as a GC lock ) so that regardless of the
	// size of the streamed-in cars nothing will disappear on us before we had
	// a chance to roots that may show up at the very end
	// This is especially important for use cases like dagger:
	//    ipfs dag import $( ... | ipfs-dagger --stdout=carfifos )
	//
	if doPinRoots {
		defer dp.bs.PinLock(dp.ctx).Unlock(dp.ctx)
	}

	// this is *not* a transaction
	// it is simply a way to relieve pressure on the blockstore
	// similar to pinner.Pin/pinner.Flush
	batch := ipld.NewBatch(dp.ctx, dp.dag)

	roots := cid.NewSet()
	var blockCount, blockBytesCount uint64

	// remember last valid block and provide a meaningful error message
	// when a truncated/mangled CAR is being imported
	importError := func(previous blocks.Block, current blocks.Block, err error) error {
		if current != nil {
			return fmt.Errorf("import failed at block %q: %w", current.Cid(), err)
		}
		if previous != nil {
			return fmt.Errorf("import failed after block %q: %w", previous.Cid(), err)
		}
		return fmt.Errorf("import failed: %w", err)
	}

	var previous blocks.Block

	car, err := gocarv2.NewBlockReader(file)
	if err != nil {
		return nil, err
	}

	for _, c := range car.Roots {
		roots.Add(c)
	}

	for {
		block, err := car.Next()
		if err != nil && err != io.EOF {
			return nil, importError(previous, block, err)
		} else if block == nil {
			break
		}
		if len(block.RawData()) > SoftBlockLimit {
			err = fmt.Errorf("produced block is over 1MiB: big blocks can't be exchanged with other peers. consider using UnixFS for automatic chunking of bigger files, or pass --allow-big-block to override")
			return nil, importError(previous, block, err)
		}

		// the double-decode is suboptimal, but we need it for batching
		nd, err := blockDecoder.DecodeNode(dp.ctx, block)
		if err != nil {
			return nil, importError(previous, block, err)
		}

		err = batch.Add(dp.ctx, nd)
		if err != nil {
			return nil, importError(previous, block, err)
		}
		blockCount++
		blockBytesCount += uint64(len(block.RawData()))
		previous = block
	}

	if err := batch.Commit(); err != nil {
		return nil, err
	}

	if doPinRoots {
		err = roots.ForEach(func(c cid.Cid) error {
			// This will trigger a full read of the DAG in the pinner, to make sure we have all blocks.
			// Ideally we would do colloring of the pinning state while importing the blocks
			// and ensure the gray bucket is empty at the end (or use the network to download missing blocks).
			block, err := dp.bs.Get(dp.ctx, c)
			if err != nil {
				return err
			}
			nd, err := blockDecoder.DecodeNode(dp.ctx, block)
			if err != nil {
				return err
			}
			err = dp.pin.Pin(dp.ctx, nd, true, "")
			if err != nil {
				return err
			}
			err = dp.pin.Flush(dp.ctx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	p := path.FromCid(car.Roots[0])
	return p, nil
}

func (dp *DagParser) Export(rootCid cid.Cid) (files.File, error) {
	return dp.ExportWithStore(dp, rootCid)
}

func (dp *DagParser) ExportWithStore(store gocar.ReadStore, rootCid cid.Cid) (files.File, error) {
	var b bytes.Buffer

	dag := gocar.Dag{Root: rootCid, Selector: selectorparse.CommonSelector_ExploreAllRecursively}
	// TraverseLinksOnlyOnce is safe for an exhaustive selector but won't be when we allow
	// arbitrary selectors here
	car := gocar.NewSelectiveCar(dp.ctx, store, []gocar.Dag{dag}, gocar.TraverseLinksOnlyOnce())
	if err := car.Write(&b); err != nil {
		return nil, err
	}

	return files.NewBytesFile(b.Bytes()), nil
}

func (dp *DagParser) ImportSdsDagLink(cid_ cid.Cid, f files.Node, doPinRoots bool) (path.Path, error) {
	logger.Debugf("Starting to import sds dag link with pin: %t for cid: %s", doPinRoots, cid_)
	fs, ok := f.(io.ReadSeeker)
	if !ok {
		return nil, fmt.Errorf("not a seeker")
	}
	// just in case
	if _, err := fs.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	fileData, err := io.ReadAll(f.(files.File))
	if err != nil {
		return nil, err
	}
	fileHash := sutil.CreateFileHash(fileData)

	blk, err := NewSdsMerkleDag(cid_, fileHash)
	if err != nil {
		return nil, err
	}

	logger.Debugf("Got recovered block for sds dag import: %s", blk.Cid())

	vbs := NewVirtualBlockStore(blk)

	eMFile, err := dp.ExportWithStore(vbs, blk.Cid())
	if err != nil {
		return nil, err
	}

	return dp.Import(eMFile, doPinRoots)
}

type VirtualBlockStore struct {
	blk blocks.Block
}

func NewVirtualBlockStore(blk blocks.Block) *VirtualBlockStore {
	return &VirtualBlockStore{
		blk: blk,
	}
}

func (vbs *VirtualBlockStore) Get(_ context.Context, c cid.Cid) (blocks.Block, error) {
	return vbs.blk, nil
}
