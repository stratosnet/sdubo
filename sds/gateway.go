package sds

import (
	"context"
	"io"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/gateway"
	"github.com/ipfs/boxo/path"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/config"
)

var _ gateway.IPFSBackend = (*SdsBlocksBackend)(nil)

type SdsBlocksBackend struct {
	b       gateway.IPFSBackend
	fetcher *Fetcher
	dag     format.DAGService
	bs      blockstore.GCBlockstore
	pin     pin.Pinner
}

func NewSdsBlockBackend(b gateway.IPFSBackend, cfg *config.Sds, dag format.DAGService, bs blockstore.GCBlockstore, pin pin.Pinner) (*SdsBlocksBackend, error) {
	fetcher, err := NewFetcher(cfg)
	if err != nil {
		return nil, err
	}
	return &SdsBlocksBackend{
		b:       b,
		fetcher: fetcher,
		dag:     dag,
		bs:      bs,
		pin:     pin,
	}, nil
}

// Logic
//
// 1. Get from ipfs by cid
// 2. If not found, get from sds
// 3. If found, try import DAG from file
// 4. If ok, load node
func (sb *SdsBlocksBackend) Get(ctx context.Context, path path.ImmutablePath, ranges ...gateway.ByteRange) (gateway.ContentPathMetadata, *gateway.GetResponse, error) {
	md, n, err := sb.b.Get(ctx, path, ranges...)
	if err != nil {
		return md, n, err
	}

	fileReader := getDynamicField(n, "bytes").(io.ReadCloser)
	fileSize := getDynamicField(n, "bytesSize").(int64)

	fileData := make([]byte, fileSize)

	if _, err = fileReader.Read(fileData); err != nil {
		return md, n, err
	}

	// NOTE: Required as we read so cursor have been moved, so content length will be missmatch
	fs := fileReader.(io.ReadSeeker)
	if _, err = fs.Seek(0, io.SeekStart); err != nil {
		return md, n, err
	}

	sdsLink, err := ParseLink(fileData)
	if err != nil {
		logger.Errorf("failed to parse sds file hash: %s", err)
		return md, n, err
	}

	sdsFileData, err := sb.fetcher.Download(sdsLink.SdsFileHash)
	if err != nil {
		logger.Errorf("failed to download file '%s' from sds: %s", sdsLink.SdsFileHash, err)
		return md, n, err
	}

	rfc := files.NewBytesFile(sdsFileData)

	_, err = NewDagParser(ctx, sb.dag, sb.bs, sb.pin).Import(rfc, true)
	if err != nil {
		return md, n, err
	}

	// nd, ok := ndMap[path.RootCid()]
	// if !ok {
	// 	return md, n, fmt.Errorf("CAR for sds not found")
	// }

	// sdsFileSize, err := rfc.Size()
	// if err != nil {
	// 	return md, n, err
	// }

	// n = gateway.NewGetResponseFromReader(rfc, sdsFileSize)

	// TODO: Refactor after and remove up block load
	md, n, err = sb.b.Get(ctx, path, ranges...)
	if err != nil {
		return md, n, err
	}

	return md, n, nil
}

func (sb *SdsBlocksBackend) GetAll(ctx context.Context, path path.ImmutablePath) (gateway.ContentPathMetadata, files.Node, error) {
	return sb.b.GetAll(ctx, path)
}

func (sb *SdsBlocksBackend) GetBlock(ctx context.Context, path path.ImmutablePath) (gateway.ContentPathMetadata, files.File, error) {
	return sb.b.GetBlock(ctx, path)
}

func (sb *SdsBlocksBackend) Head(ctx context.Context, path path.ImmutablePath) (gateway.ContentPathMetadata, *gateway.HeadResponse, error) {
	return sb.b.Head(ctx, path)
}

func (sb *SdsBlocksBackend) ResolvePath(ctx context.Context, path path.ImmutablePath) (gateway.ContentPathMetadata, error) {
	return sb.b.ResolvePath(ctx, path)
}

func (sb *SdsBlocksBackend) GetCAR(ctx context.Context, p path.ImmutablePath, params gateway.CarParams) (gateway.ContentPathMetadata, io.ReadCloser, error) {
	return sb.b.GetCAR(ctx, p, params)
}

func (sb *SdsBlocksBackend) IsCached(ctx context.Context, path path.Path) bool {
	return sb.b.IsCached(ctx, path)
}

func (sb *SdsBlocksBackend) GetIPNSRecord(ctx context.Context, cid cid.Cid) ([]byte, error) {
	return sb.b.GetIPNSRecord(ctx, cid)
}

func (sb *SdsBlocksBackend) ResolveMutable(ctx context.Context, path path.Path) (path.ImmutablePath, time.Duration, time.Time, error) {
	return sb.b.ResolveMutable(ctx, path)
}

func (sb *SdsBlocksBackend) GetDNSLinkRecord(ctx context.Context, hostname string) (path.Path, error) {
	return sb.b.GetDNSLinkRecord(ctx, hostname)
}
