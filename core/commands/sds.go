package commands

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/misc/sutil"
	"github.com/ipfs/kubo/sds"
	"github.com/ipfs/kubo/sg"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	mh "github.com/multiformats/go-multihash"
)

var (
	spfsUserIdOption    = cmds.StringOption(sds.OptionSpfsUserId, "Spfs user id for user management.")
	spfsProjectIdOption = cmds.IntOption(sds.OptionSpfsProjectId, "Spfs project id for user management.")
	spfsPrivKeyOption   = cmds.StringOption(sds.OptionSpfsPrivKey, "Spfs user priv key for signing.")
)

var logger = logging.Logger("core/commands/sds")

// TODO: Add pin per user, currently always pin
func addSdsCar(req *cmds.Request, cfg *config.Config, api iface.CoreAPI, cid_ cid.Cid, nd *core.IpfsNode, pin bool, onlyHash bool) (path.ImmutablePath, error) {
	// TODO: Is it still maybe possible?
	if onlyHash {
		return path.ImmutablePath{}, fmt.Errorf("simulate feature disabled")
	}

	f, err := sds.NewDagParser(req.Context, nd.Blockstore, nd.Pinning).Export(cid_)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	fSize, err := f.Size()
	if err != nil {
		return path.ImmutablePath{}, err
	}

	sOpts := []options.SdsOption{
		options.Sds.HashOnly(onlyHash),
	}

	projectId, _ := req.Options[sds.OptionSpfsProjectId].(int)
	privKey, _ := req.Options[sds.OptionSpfsPrivKey].(string)
	if privKey != "" {
		sOpts = append(sOpts, options.Sds.PrivKey(privKey))
	}

	sdsFileHash, err := api.Sds().Upload(req.Context, f, sOpts...)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	mapFile, err := api.Sds().Link(req.Context, cid_, sdsFileHash, sOpts...)
	if err != nil {
		return path.ImmutablePath{}, err
	}

	mhtype := cfg.Import.HashFunction.WithDefault(config.DefaultHashFunction)
	mhtval, ok := mh.Names[mhtype]
	if !ok {
		return path.ImmutablePath{}, fmt.Errorf("unrecognized multihash function: %s", mhtype)
	}

	var (
		sPath path.ImmutablePath
	)

	// for 0 means add api, so we relly on incomming cid_ param to detect behaviour
	if cid_.Version() == 0 {
		chunker := cfg.Import.UnixFSChunker.WithDefault(config.DefaultUnixFSChunker)
		hashFunStr := cfg.Import.HashFunction.WithDefault(config.DefaultHashFunction)
		hashFunCode, ok := mh.Names[strings.ToLower(hashFunStr)]
		if !ok {
			return path.ImmutablePath{}, fmt.Errorf("unrecognized hash function: %q", strings.ToLower(hashFunStr))
		}

		opts := []options.UnixfsAddOption{
			options.Unixfs.Hash(hashFunCode),

			options.Unixfs.Inline(false),
			options.Unixfs.InlineLimit(32),

			options.Unixfs.Chunker(chunker),

			options.Unixfs.Pin(true),
			options.Unixfs.HashOnly(onlyHash),
			options.Unixfs.FsCache(false),
			options.Unixfs.Nocopy(false),

			options.Unixfs.Progress(false),
			options.Unixfs.Silent(true),

			options.Unixfs.PreserveMode(false),
			options.Unixfs.PreserveMtime(false),
			options.Unixfs.CidVersion(0),
		}

		sPath, err = api.Unixfs().Add(req.Context, mapFile, opts...)
		if err != nil {
			return path.ImmutablePath{}, err
		}
	} else {
		opts := []options.BlockPutOption{
			options.Block.Hash(mhtval, -1),
			options.Block.CidCodec("raw"),
			options.Block.Format(""),
			options.Block.Pin(true),
		}

		blockStat, err := api.Block().Put(req.Context, mapFile, opts...)
		if err != nil {
			return path.ImmutablePath{}, err
		}

		if err := cmdutils.CheckBlockSize(req, uint64(blockStat.Size())); err != nil {
			return path.ImmutablePath{}, err
		}

		sPath = blockStat.Path()
	}

	if cfg.Sg.Enabled {
		reporter := sg.NewReporter(sg.NewClient(cfg.Sg.URI), nd.MFSRepo.MFSDS)
		if err := reporter.Store(req.Context, sPath.RootCid().String(), sg.ReportFileInfo{
			ProjectID:   projectId,
			IPFSCid:     cid_,
			SdsCid:      sPath.RootCid(),
			SdsFileHash: sdsFileHash,
			FileSize:    uint64(fSize),
		}); err != nil {
			return path.ImmutablePath{}, err
		}
	}

	// // NOTE: linking after to main mfs tree for proper gc
	// // should be replaced somehow in future
	// filesRoot, err := nd.GetMFSRoot("")
	// if err != nil {
	// 	return path.ImmutablePath{}, err
	// }

	// nodeAdded, err := api.Dag().Get(req.Context, sPath.RootCid())
	// if err != nil {
	// 	return path.ImmutablePath{}, err
	// }

	// _ = mfs.PutNode(filesRoot, fmt.Sprintf("/%s", sPath.RootCid()), nodeAdded)

	return sPath, nil

}

func getSdsCarOrResolve(nd *core.IpfsNode, cfg *config.Config, ctx context.Context, api iface.CoreAPI, p path.Path, opts ...options.SdsOption) (files.Node, error) {
	if cfg.Sg.Enabled {
		defer func() {
			reporter := sg.NewReporter(sg.NewClient(cfg.Sg.URI), nd.MFSRepo.MFSDS)
			path_, err := path.NewImmutablePath(p)
			if err != nil {
				return
			}
			if err := reporter.Notify(ctx, path_.RootCid().String()); err != nil {
				logger.Warn("failed to report sg volume", "err", err)
			}
		}()
	}

	ctxUfs, _ := context.WithTimeout(ctx, time.Duration(sds.SpfsGatewayBlockTimeout)*time.Second)
	// NOTE: Check first if file exists in ipfs
	f, err := api.Unixfs().Get(ctxUfs, p)

	// Not exist, trying to get from sds
	if err != nil {
		if !cfg.Sds.Enabled {
			return nil, err
		}

		logger.Debugf("Downloading from sds by path: %s", p)
		sf, err := api.Sds().Download(ctx, p, opts...)
		if err != nil {
			return nil, err
		}

		f = sf.(files.Node)
	} else if cfg.Sds.Enabled {
		// in case file found on ipfs, check if it is a mapping file and get original car file
		// NOTE: Risk of broke API with mailware map file?
		mFile, ok := f.(files.File)
		if ok {
			logger.Debugf("Parsing sds link from map file")
			np, err := api.Sds().Parse(ctx, mFile)
			if err == nil {
				ctxUfs, _ := context.WithTimeout(ctx, time.Duration(sds.SpfsGatewayBlockTimeout)*time.Second)
				logger.Debugf("Trying to get original file from dag store for path: %s", np)
				f, err = api.Unixfs().Get(ctxUfs, np)
				if err == nil {
					return f, nil
				}
				logger.Debugf("Origin not found, downloading from sds by path: %s", p)
				sf, err := api.Sds().Download(ctx, p, opts...)
				if err != nil {
					return nil, err
				}

				f = sf.(files.Node)
			}
		}
	}

	isCar, _ := sutil.IsCAR(f)
	logger.Debugf("Get car file: %t for cid: %s", isCar, p)
	// after fetched car, we need to be sure it is a car, otherwise handle it as ipfs file
	if isCar {
		// TODO: Add a way to import only if it is not exists
		dp := sds.NewDagParser(ctx, nd.Blockstore, nd.Pinning)
		sdsP, err := dp.Import(f.(files.File), false)
		logger.Debugf("Imported car for original file on path: %s", sdsP)
		if err != nil {
			return nil, err
		}

		sdsP, err = sutil.ExtendPath(sdsP, p)
		if err != nil {
			return nil, err
		}

		cid_, err := cid.Parse(sdsP.Segments()[1])
		if err != nil {
			return nil, err
		}

		if _, err = dp.ImportSdsDagLink(cid_, f, true); err != nil {
			return nil, err
		}
		logger.Debugf("Imported link for original file on cid: %s", cid_)

		logger.Debugf("Retrieving original file from dag store for path: %s", sdsP)
		f, err = api.Unixfs().Get(ctx, sdsP)
		logger.Debugf("Dag store file err on resp: %v", err)
		if err != nil {
			return nil, err
		}
	}

	return f, nil
}

// sdsTmpRecreateShareLink is TMP fix sync for GC collected files
func sdsTmpRecreateShareLink(api iface.CoreAPI, ctx context.Context, req *cmds.Request, c cid.Cid) error {
	// NOTE: Tmp fix for files which was GC and get them from sds
	sls := sds.NewShareLinkService(nil, nil)
	privKey, _ := req.Options[sds.OptionSpfsPrivKey].(string)

	offlineApi, err := api.WithOptions(options.Api.Offline(true))
	if err != nil {
		return err
	}

	ff, err := offlineApi.Unixfs().Get(ctx, path.FromCid(c))
	fmt.Println("get tmp nd", ff, err)
	if err != nil {
		return err
	}

	mFile, ok := ff.(files.File)
	if ok {
		fsize, err := mFile.Size()
		if err != nil {
			return err
		}

		fileData := make([]byte, fsize)
		_, err = io.ReadFull(mFile, fileData)
		if err != nil {
			return err
		}

		sdsLink, err := sds.ParseLink(fileData)
		if err != nil {
			return err
		}

		fmt.Println("sdsLink total cid", c.String())
		fmt.Println("sdsLink.OriginalCid", sdsLink.OriginalCid)
		fmt.Println("sdsLink.SdsFileHash", sdsLink.SdsFileHash)

		if err := sls.Add(ctx, cid.MustParse(sdsLink.OriginalCid), sdsLink.SdsFileHash, privKey); err != nil {
			return err
		}
	}
	return nil
}
