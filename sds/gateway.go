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

	// NOTE: Check first if file exists in ipfs
	md, n, err := sb.b.Get(ctx, path_, ranges...)
	fmt.Printf("SdsBlocksBackend Get path_ %+v\n", path_)
	fmt.Printf("SdsBlocksBackend Get md %+v\n", md)
	fmt.Printf("SdsBlocksBackend Get n %+v\n", n)
	fmt.Printf("SdsBlocksBackend Get err %+v\n", err)
	// Not exist, trying to get from sds
	if err != nil {
		if !sb.cfg.Enabled {
			return md, n, err
		}

		// TODO: Maybe to get from ipfs also first?
		shareLink := fwtypes.SetShareLink(path_.Segments()[1], "")

		// no care of error
		fileData, _ = sb.fetcher.DownloadFromShare(shareLink.String())
		// in this case we should pin to store into local block tree
		doPinRoots = true
	} else if sb.cfg.Enabled {
		// in case file found on ipfs, check if it is a mapping file and get original car file
		// getting file data from gateway
		fileData, errS = readAndResetGatewayResponse(n)
		fmt.Println("fileData errS", errS)
		if errS == nil {
			originalCid, errS := ParseLink(fileData)
			fmt.Printf("originalCid %+v\n", originalCid)
			if errS == nil {
				oPath, errS := path.NewPath("/ipfs/" + originalCid.String())
				if err != nil {
					return gateway.ContentPathMetadata{}, nil, errS
				}
				path_, errS = path.NewImmutablePath(oPath)
				if errS != nil {
					return gateway.ContentPathMetadata{}, nil, errS
				}
				fmt.Printf("path_ %+v\n", path_)
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

	fmt.Println("fileData", fileData)

	isCar, _ := IsCAR(files.NewBytesFile(fileData))
	fmt.Printf("isCar %+v\n", isCar)
	if isCar {
		sdsP, errS := NewDagParser(ctx, sb.dag, sb.bs, sb.pin).Import(files.NewBytesFile(fileData), doPinRoots)
		if errS != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		fmt.Printf("sdsP (before) %+v\n", sdsP)
		sdsP, errS = ModifySdsCARPath(sdsP, path_)
		fmt.Printf("sdsP (after) %+v\n", sdsP)
		if errS != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		path_, errS = path.NewImmutablePath(sdsP)
		if errS != nil {
			return gateway.ContentPathMetadata{}, nil, errS
		}

		md, n, err = sb.b.Get(ctx, path_, ranges...)
		fmt.Printf("SdsBlocksBackend Get (CAR) path_ %+v\n", path_)
		fmt.Printf("SdsBlocksBackend Get (CAR) md %+v\n", md)
		fmt.Printf("SdsBlocksBackend Get (CAR) n %+v\n", n)
		fmt.Printf("SdsBlocksBackend Get (CAR) err %+v\n", err)
		if err != nil {
			return md, n, err
		}
	}

	fmt.Println("md total", md)
	fmt.Println("n total", n)
	fmt.Println("err total", err)

	return md, n, err
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
