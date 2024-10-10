package sds

import (
	"context"
	"fmt"
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
	fwtypes "github.com/stratosnet/sds/framework/types"
)

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
	fetcher, err := NewFetcher(cfg)
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
	fileReader := getDynamicField(n, "bytes").(io.ReadCloser)
	fileSize := getDynamicField(n, "bytesSize").(int64)

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
	)

	// NOTE: Check first if file exists in ipfs
	_, n, err := sb.b.Get(ctx, path_, ranges...)
	// Not exist, trying to get from sds
	if err != nil {
		if !sb.cfg.Enabled {
			return gateway.ContentPathMetadata{}, nil, err
		}

		shareLink := fwtypes.SetShareLink(path_.Segments()[1], "")

		fileData, err = sb.fetcher.DownloadFromShare(shareLink.String())
		if err != nil {
			logger.Errorf("failed to download share car file '%s' from sds: %s", shareLink.String(), err)
			return gateway.ContentPathMetadata{}, nil, err
		}

		// in this case we should pin to store into local block tree
		doPinRoots = true
	} else if sb.cfg.Enabled {
		// in case file found on ipfs, check if it is a mapping file and get original car file
		// NOTE: Risk of broke API with mailware map file?

		// getting file data from gateway
		fileData, err = readAndResetGatewayResponse(n)
		if err != nil {
			return gateway.ContentPathMetadata{}, nil, err
		}

		originalCid, err := ParseLink(fileData)
		fmt.Printf("originalCid %+v\n", originalCid)
		if err == nil {
			oPath, err := path.NewPath("/ipfs/" + originalCid.String())
			if err != nil {
				return gateway.ContentPathMetadata{}, nil, err
			}
			path_, err := path.NewImmutablePath(oPath)
			if err != nil {
				return gateway.ContentPathMetadata{}, nil, err
			}
			fmt.Printf("path_ %+v\n", path_)
			_, n, err = sb.b.Get(ctx, path_, ranges...)
			if err != nil {
				return gateway.ContentPathMetadata{}, nil, err
			}
			// update gateway content
			fileData, err = readAndResetGatewayResponse(n)
			if err != nil {
				return gateway.ContentPathMetadata{}, nil, err
			}
		}
	}

	// TODO: Problematic file content read
	isCar, _ := IsCAR(files.NewBytesFile(fileData))
	fmt.Printf("isCar %+v\n", isCar)
	if isCar {
		sdsP, err := NewDagParser(ctx, sb.dag, sb.bs, sb.pin).Import(files.NewBytesFile(fileData), doPinRoots)
		if err != nil {
			return gateway.ContentPathMetadata{}, nil, err
		}

		sdsP, err = ModifySdsCARPath(sdsP, path_)
		if err != nil {
			return gateway.ContentPathMetadata{}, nil, err
		}

		path_, err = path.NewImmutablePath(sdsP)
		if err != nil {
			return gateway.ContentPathMetadata{}, nil, err
		}

		return sb.b.Get(ctx, path_, ranges...)
	}

	rfc := files.NewBytesFile(fileData)
	fmt.Printf("rfc %+v\n", rfc)
	fileSize, err := rfc.Size()
	if err != nil {
		return gateway.ContentPathMetadata{}, nil, err
	}

	n = gateway.NewGetResponseFromReader(rfc, fileSize)
	md, err := sb.b.ResolvePath(ctx, path_)

	fmt.Println("doPinRoots", doPinRoots)
	fmt.Println("fileData", fileData)

	fmt.Printf("GET n %+v\n", n)
	fmt.Printf("GET md %+v\n", md)
	if err != nil {
		return gateway.ContentPathMetadata{}, nil, err
	}

	return md, n, nil
}

// func (sb *SdsBlocksBackend) GetLegacy(ctx context.Context, path path.ImmutablePath, ranges ...gateway.ByteRange) (gateway.ContentPathMetadata, *gateway.GetResponse, error) {
// 	md, n, err := sb.b.Get(ctx, path, ranges...)
// 	if err != nil {
// 		return md, n, err
// 	}

// 	fileReader := getDynamicField(n, "bytes").(io.ReadCloser)
// 	fileSize := getDynamicField(n, "bytesSize").(int64)

// 	fileData := make([]byte, fileSize)

// 	if _, err = fileReader.Read(fileData); err != nil {
// 		return md, n, err
// 	}

// 	// NOTE: Required as we read so cursor have been moved, so content length will be missmatch
// 	fs := fileReader.(io.ReadSeeker)
// 	if _, err = fs.Seek(0, io.SeekStart); err != nil {
// 		return md, n, err
// 	}

// 	originalCid, err := ParseLink(fileData)
// 	if err != nil {
// 		logger.Errorf("failed to parse sds file hash: %s", err)
// 		return md, n, err
// 	}

// 	shareLink := fwtypes.SetShareLink(originalCid.String(), "")

// 	sdsFileData, err := sb.fetcher.DownloadFromShare(shareLink.String())
// 	if err != nil {
// 		logger.Errorf("failed to download share car file '%s' from sds: %s", shareLink.String(), err)
// 		return md, n, err
// 	}

// 	rfc := files.NewBytesFile(sdsFileData)

// 	_, err = NewDagParser(ctx, sb.dag, sb.bs, sb.pin).Import(rfc, true)
// 	if err != nil {
// 		return md, n, err
// 	}

// 	// nd, ok := ndMap[path.RootCid()]
// 	// if !ok {
// 	// 	return md, n, fmt.Errorf("CAR for sds not found")
// 	// }

// 	// sdsFileSize, err := rfc.Size()
// 	// if err != nil {
// 	// 	return md, n, err
// 	// }

// 	// n = gateway.NewGetResponseFromReader(rfc, sdsFileSize)

// 	// TODO: Refactor after and remove up block load
// 	md, n, err = sb.b.Get(ctx, path, ranges...)
// 	if err != nil {
// 		return md, n, err
// 	}

// 	return md, n, nil
// }

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
