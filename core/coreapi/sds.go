package coreapi

import (
	"context"
	"fmt"
	"io"

	blockservice "github.com/ipfs/boxo/blockservice"
	bstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/files"
	filestore "github.com/ipfs/boxo/filestore"
	merkledag "github.com/ipfs/boxo/ipld/merkledag"
	dagtest "github.com/ipfs/boxo/ipld/merkledag/test"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	"github.com/ipfs/boxo/mfs"
	"github.com/ipfs/boxo/path"
	cidutil "github.com/ipfs/go-cidutil"
	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	options "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/core/coreunix"
	"github.com/ipfs/kubo/sds"
	sdsprotos "github.com/ipfs/kubo/sds/protos"
	"github.com/ipfs/kubo/tracing"
	fwtypes "github.com/stratosnet/sds/framework/types"
	"go.opentelemetry.io/otel/attribute"
)

type SdsAPI CoreAPI

// Link a path with sds, adds it to the blockstore,
// and returns the key representing that node.
func (api *SdsAPI) Link(ctx context.Context, sdsLink *sdsprotos.SdsLinker, opts ...options.UnixfsAddOption) (path.ImmutablePath, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.SdsAPI", "Link")
	defer span.End()

	settings, prefix, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	span.SetAttributes(
		attribute.String("chunker", settings.Chunker),
		attribute.Int("cidversion", settings.CidVersion),
		attribute.Bool("inline", settings.Inline),
		attribute.Int("inlinelimit", settings.InlineLimit),
		attribute.Bool("rawleaves", settings.RawLeaves),
		attribute.Bool("rawleavesset", settings.RawLeavesSet),
		attribute.Int("layout", int(settings.Layout)),
		attribute.Bool("pin", settings.Pin),
		attribute.Bool("onlyhash", settings.OnlyHash),
		attribute.Bool("fscache", settings.FsCache),
		attribute.Bool("nocopy", settings.NoCopy),
		attribute.Bool("silent", settings.Silent),
		attribute.Bool("progress", settings.Progress),
	)

	cfg, err := api.repo.Config()
	if err != nil {
		return path.ImmutablePath{}, err
	}

	// check if repo will exceed storage limit if added
	// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
	// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
	//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
	//	res.SetError(err, cmds.ErrNormal)
	//	return
	//}

	if settings.NoCopy && !(cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled) {
		return path.ImmutablePath{}, fmt.Errorf("either the filestore or the urlstore must be enabled to use nocopy, see: https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-filestore")
	}

	addblockstore := api.blockstore
	if !(settings.FsCache || settings.NoCopy) {
		addblockstore = bstore.NewGCBlockstore(api.baseBlocks, api.blockstore)
	}
	exch := api.exchange
	pinning := api.pinning

	if settings.OnlyHash {
		// setup a /dev/null pipeline to simulate adding the data
		dstore := dssync.MutexWrap(ds.NewNullDatastore())
		bs := bstore.NewBlockstore(dstore, bstore.WriteThrough())
		addblockstore = bstore.NewGCBlockstore(bs, nil) // gclocker will never be used
		exch = nil                                      // exchange will never be used
		pinning = nil                                   // pinner will never be used
	}

	bserv := blockservice.New(addblockstore, exch) // hash security 001
	dserv := merkledag.NewDAGService(bserv)

	// add a sync call to the DagService
	// this ensures that data written to the DagService is persisted to the underlying datastore
	// TODO: propagate the Sync function from the datastore through the blockstore, blockservice and dagservice
	var syncDserv *syncDagService
	if settings.OnlyHash {
		syncDserv = &syncDagService{
			DAGService: dserv,
			syncFn:     func() error { return nil },
		}
	} else {
		syncDserv = &syncDagService{
			DAGService: dserv,
			syncFn: func() error {
				rds := api.repo.Datastore()
				if err := rds.Sync(ctx, bstore.BlockPrefix); err != nil {
					return err
				}
				return rds.Sync(ctx, filestore.FilestorePrefix)
			},
		}
	}

	fileAdder, err := coreunix.NewAdder(ctx, pinning, addblockstore, syncDserv)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	fileAdder.Chunker = settings.Chunker
	if settings.Events != nil {
		fileAdder.Out = settings.Events
		fileAdder.Progress = settings.Progress
	}
	fileAdder.Pin = settings.Pin && !settings.OnlyHash
	fileAdder.Silent = settings.Silent
	fileAdder.RawLeaves = settings.RawLeaves
	fileAdder.NoCopy = settings.NoCopy
	fileAdder.CidBuilder = prefix

	switch settings.Layout {
	case options.BalancedLayout:
		// Default
	case options.TrickleLayout:
		fileAdder.Trickle = true
	default:
		return path.ImmutablePath{}, fmt.Errorf("unknown layout: %d", settings.Layout)
	}

	if settings.Inline {
		fileAdder.CidBuilder = cidutil.InlineBuilder{
			Builder: fileAdder.CidBuilder,
			Limit:   settings.InlineLimit,
		}
	}

	if settings.OnlyHash {
		md := dagtest.Mock()
		emptyDirNode := ft.EmptyDirNode()
		// Use the same prefix for the "empty" MFS root as for the file adder.
		err := emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
		if err != nil {
			return path.ImmutablePath{}, err
		}
		mr, err := mfs.NewRoot(ctx, md, emptyDirNode, nil)
		if err != nil {
			return path.ImmutablePath{}, err
		}

		fileAdder.SetMfsRoot(mr)
	}

	mapFile, err := sds.NewSdsFile(sdsLink)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	_, err = api.sdsFetcher.CreateShareLink(sdsLink.SdsFileHash, sdsLink.OriginalCid)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	nd, err := fileAdder.AddAllAndPin(ctx, mapFile)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	if !settings.OnlyHash {
		if err := api.provider.Provide(nd.Cid()); err != nil {
			return path.ImmutablePath{}, err
		}
	}

	return path.FromCid(nd.Cid()), nil
}

// Add imports the data from the reader into sds store chunks
func (api *SdsAPI) Upload(ctx context.Context, file_ files.File, opts ...options.UnixfsAddOption) (string, error) {
	fileData, err := io.ReadAll(file_)
	if err != nil {
		return "", err
	}

	return api.sdsFetcher.Upload(fileData)
}

func (api *SdsAPI) Parse(ctx context.Context, file_ files.File) (*sdsprotos.SdsLinker, error) {
	fsize, err := file_.Size()
	if err != nil {
		return nil, err
	}

	fileData := make([]byte, fsize)
	_, err = io.ReadFull(file_, fileData)
	if err != nil {
		return nil, err
	}

	return sds.ParseLink(fileData)
}

func (api *SdsAPI) Download(ctx context.Context, p path.Path) (files.File, error) {
	fmt.Println("p.Segments()", p.Segments())
	fmt.Println("p.Segments()[1]", p.Segments()[1])
	shareLink := fwtypes.SetShareLink(p.Segments()[1], "")
	fmt.Println("shareLink", shareLink)
	fileData, err := api.sdsFetcher.DownloadFromShare(shareLink.String())
	if err != nil {
		return nil, err
	}

	rfc := files.NewBytesFile(fileData)

	return rfc, nil
}
