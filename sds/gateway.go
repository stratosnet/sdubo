package sds

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/gateway"
	"github.com/ipfs/boxo/path"
	pin "github.com/ipfs/boxo/pinning/pinner"
	"github.com/ipfs/go-cid"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/config"
	fwtypes "github.com/stratosnet/sds/framework/types"
)

const SpfsGatewayBlockTimeout = 30

var _ gateway.IPFSBackend = (*SdsBlocksBackend)(nil)

type SdsBlocksBackend struct {
	b       gateway.IPFSBackend
	cfg     *config.Sds
	fetcher *Fetcher
	dag     format.DAGService
	bs      blockstore.GCBlockstore
	pin     pin.Pinner
}

func NewSdsBlockBackend(b gateway.IPFSBackend, cfg *config.Sds, dag format.DAGService, bs blockstore.GCBlockstore, pin pin.Pinner) (*SdsBlocksBackend, error) {
	fetcher, err := NewFetcher(cfg, true)
	if err != nil {
		return nil, err
	}

	return &SdsBlocksBackend{
		b:       b,
		cfg:     cfg,
		fetcher: fetcher,
		dag:     dag,
		bs:      bs,
		pin:     pin,
	}, nil
}

func readAndResetGatewayResponse(n *gateway.GetResponse) ([]byte, error) {
	fileReader, ok := getDynamicField(n, "bytes").(io.ReadCloser)
	if !ok {
		return []byte{}, fmt.Errorf("not a file reader")
	}
	fileSize, ok := getDynamicField(n, "bytesSize").(int64)
	if !ok {
		return []byte{}, fmt.Errorf("no file size")
	}

	fileData := make([]byte, fileSize)

	if _, err := fileReader.Read(fileData); err != nil {
		return []byte{}, err
	}

	// NOTE: Required as we read so cursor have been moved, so content length will be missmatch
	fs, ok := fileReader.(io.ReadSeeker)
	if ok {
		if _, err := fs.Seek(0, io.SeekStart); err != nil {
			return []byte{}, err
		}
	}

	return fileData, nil
}

// TODO: Fix panic on folder
// Logic
//
// 1. Get from ipfs by cid
// 2. If not found, get from sds
// 3. If found, try import DAG from file
// 4. If ok, load node
func (sb *SdsBlocksBackend) Get(ctx context.Context, path_ path.ImmutablePath, ranges ...gateway.ByteRange) (gateway.ContentPathMetadata, *gateway.GetResponse, error) {
	var (
		doPinRoots = false
		fileData   []byte
		errS       error
	)

	ctx2, cancelFn := context.WithTimeout(ctx, time.Duration(SpfsGatewayBlockTimeout)*time.Second)
	defer cancelFn()

	// NOTE: Check first if file exists in ipfs
	md, n, err := sb.b.Get(ctx2, path_, ranges...)

	// Not exist, trying to get from sds
	if err != nil {
		skipPath := strings.Contains(err.Error(), "index.html") ||
			strings.Contains(err.Error(), "favicon.ico")
		if !sb.cfg.Enabled || skipPath {
			return md, n, err
		}

		c, errS := cid.Parse(path_.Segments()[1])
		if errS != nil {
			return md, n, err
		}

		if c.Type() != cid.DagProtobuf {
			return md, n, err
		}

		if c.Version() == 1 {
			npath_, errS := ExtendPath(path.FromCid(cid.NewCidV0(c.Hash())), path_)
			if errS != nil {
				return gateway.ContentPathMetadata{}, nil, errS
			}

			path_, errS = path.NewImmutablePath(npath_)
			if errS != nil {
				return gateway.ContentPathMetadata{}, nil, errS
			}
		}

		// TODO: Maybe to get from ipfs also first?
		shareLink := fwtypes.SetShareLink(path_.Segments()[1], "")

		// no care of error
		// TODO: Add pk from sg
		fileData, _ = sb.fetcher.DownloadFromShare("", shareLink.String())
		// in this case we should pin to store into local block tree
		doPinRoots = true
	} else if sb.cfg.Enabled {
		// in case file found on ipfs, check if it is a mapping file and get original car file
		// getting file data from gateway
		fileData, errS = readAndResetGatewayResponse(n)
		if errS == nil {
			originalCid, errS := ParseLink(fileData)
			if errS == nil {
				oPath, errS := path.NewPath("/ipfs/" + originalCid.String())
				if errS != nil {
					return gateway.ContentPathMetadata{}, nil, errS
				}
				path_, errS = path.NewImmutablePath(oPath)
				if errS != nil {
					return gateway.ContentPathMetadata{}, nil, errS
				}
				md, n, err = sb.b.Get(ctx, path_, ranges...)
				if err != nil {
					return md, n, err
				}
				// update gateway content
				fileData, errS = readAndResetGatewayResponse(n)
				if errS != nil {
					return md, n, nil
				}
			}
		}
	}

	isCar, _ := IsCAR(files.NewBytesFile(fileData))
	if isCar {
		dp := NewDagParser(ctx, sb.dag, sb.bs, sb.pin)
		// TODO: Add a way to import only if it is not exists
		sdsP, errS := dp.Import(files.NewBytesFile(fileData), doPinRoots)
		if errS != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		sdsP, errS = ExtendPath(sdsP, path_)
		if errS != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		cid_, err := cid.Parse(sdsP.Segments()[1])
		if err != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		// TODO: Add a way to import only if it is not exists
		if _, err = dp.ImportSdsDagLink(cid_, files.NewBytesFile(fileData)); err != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		path_, errS = path.NewImmutablePath(sdsP)
		if errS != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		md, n, err = sb.b.Get(ctx, path_, ranges...)
		if err != nil {
			return md, n, err
		}
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
