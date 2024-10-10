package commands

import (
	"context"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/sds"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
)

func getCarOrResolve(nd *core.IpfsNode, cfg *config.Config, ctx context.Context, api iface.CoreAPI, p path.Path) (files.Node, error) {
	var (
		doPinRoots = false
	)
	// NOTE: Check first if file exists in ipfs
	f, err := api.Unixfs().Get(ctx, p)
	// Not exist, trying to get from sds
	if err != nil {
		if !cfg.Sds.Enabled {
			return nil, err
		}

		sf, err := api.Sds().Download(ctx, p)
		if err != nil {
			return nil, err
		}

		f = sf.(files.Node)
		// in this case we should pin to store into local block tree
		doPinRoots = true
	} else {
		// in case file found on ipfs, check if it is a mapping file and get original car file
		// NOTE: Risk of broke API with mailware map file?
		mFile, ok := f.(files.File)
		if ok {
			p, err := api.Sds().Parse(ctx, mFile)
			if err == nil {
				f, err = api.Unixfs().Get(ctx, p)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	isCar, _ := sds.IsCAR(f)
	// after fetched car, we need to be sure it is a car, otherwise handle it as ipfs file
	if isCar {
		// offline api after to ensure we do not reach out to the network for any reason
		api, err = api.WithOptions(options.Api.Offline(true))
		if err != nil {
			return nil, err
		}

		sdsP, err := sds.NewDagParser(ctx, api.Dag(), nd.Blockstore, nd.Pinning).Import(f.(files.File), doPinRoots)
		if err != nil {
			return nil, err
		}

		sdsP, err = sds.ModifySdsCARPath(sdsP, p)
		if err != nil {
			return nil, err
		}

		f, err = api.Unixfs().Get(ctx, sdsP)
		if err != nil {
			return nil, err
		}
	}

	return f, nil
}
