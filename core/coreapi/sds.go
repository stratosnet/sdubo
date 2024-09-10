package coreapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

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
	"github.com/ipfs/kubo/tracing"
	rpc_api "github.com/stratosnet/sds/pp/api/rpc"
	"go.opentelemetry.io/otel/attribute"
)

type sdsLinker struct {
	OriginalCID string `json:"originalCID,omitempty"`
	SdsFileHash string `json:"sdsFileHash,omitempty"`
}

func NewSdsFile(cidLink path.ImmutablePath, sdsFileHash string) (files.Node, error) {
	link := &sdsLinker{
		OriginalCID: cidLink.RootCid().String(),
		SdsFileHash: sdsFileHash,
	}
	b, err := json.Marshal(link)
	if err != nil {
		return nil, err
	}
	rfc := files.NewBytesFile(b)
	// mapFile := files.NewMapDirectory(map[string]files.Node{
	// 	"sdsMapper": rfc,
	// })
	return rfc, nil
}

type SdsAPI CoreAPI

// Link a path with sds, adds it to the blockstore,
// and returns the key representing that node.
func (api *SdsAPI) Link(ctx context.Context, cidLink path.ImmutablePath, sdsFileHash string, opts ...options.UnixfsAddOption) (path.ImmutablePath, error) {
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

	mapFile, err := NewSdsFile(cidLink, sdsFileHash)
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

func randomFileName(size int, ext string) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x.%s", b, ext), nil
}

// Add imports the data from the reader into sds store chunks
func (api *SdsAPI) Add(ctx context.Context, files_ files.Node, opts ...options.UnixfsAddOption) (string, error) {
	// ctx, span := tracing.Span(ctx, "CoreAPI.SdsAPI", "Add")
	// defer span.End()

	// settings, prefix, err := options.UnixfsAddOptions(opts...)
	// if err != nil {
	// 	return "", err
	// }

	// TODO: Move to conf
	wallet, err := sds.NewSdsSecp256k1Wallet("0xf4a2b939592564feb35ab10a8e04f6f2fe0943579fb3c9c33505298978b74893")
	if err != nil {
		return "", err
	}
	// TODO: Move to conf
	rpc, err := sds.NewRpc("http://0.0.0.0:18281")
	if err != nil {
		return "", err
	}

	var file files.File

	switch f := files_.(type) {
	case files.File:
		file = f
	default:
		return "", fmt.Errorf("not a file, abort")
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	fileHash := sds.CreateFileHash(fileData)

	oz, err := rpc.GetOzone(wallet)
	if err != nil {
		return "", err
	}

	// TODO: How to get file name?
	fileName, err := randomFileName(16, "txt")
	if err != nil {
		return "", err
	}

	res, err := rpc.RequestUpload(wallet, oz.SequenceNumber, fileName, fileHash, len(fileData))
	if err != nil {
		// this is sp error, means that file already exist and uploaded, so we could just link
		if strings.Contains(err.Error(), "Same file with the name") {
			return fileHash, nil
		}
		return "", err
	}
	if res.Return != "1" {
		return "", fmt.Errorf("failed sp upload with error: %s", res.Return)
	}

	for res.Return == rpc_api.UPLOAD_DATA {
		chunkData := make([]byte, *res.OffsetEnd-*res.OffsetStart)
		copy(chunkData, fileData[*res.OffsetStart:*res.OffsetEnd])
		fileChunk := base64.StdEncoding.EncodeToString(chunkData)
		if err != nil {
			return "", err
		}

		res, err = rpc.UploadData(wallet, oz.SequenceNumber, fileHash, fileChunk)
		if err != nil {
			return "", err
		}
	}

	return fileHash, nil
}
