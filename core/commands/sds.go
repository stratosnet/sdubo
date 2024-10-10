package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
	fmt.Println("unixfs get", f)
	fmt.Println("unixfs get err", err)
	// Not exist, trying to get from sds
	if err != nil {
		if !cfg.Sds.Enabled {
			return nil, err
		}

		sf, err := api.Sds().Download(ctx, p)
		fmt.Println("sds download err", err)
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
			fmt.Printf("getCarOrResolve p (original) %+v\n", p)
			if err == nil {
				f, err = api.Unixfs().Get(ctx, p)
				if err != nil {
					return nil, err
				}
				fmt.Printf("getCarOrResolve f (car) %+v\n", f)
			}
		}
	}

	fmt.Printf("getCarOrResolve p (requested) %+v\n", p)

	isCar, _ := sds.IsCAR(f)
	// after fetched car, we need to be sure it is a car, otherwise handle it as ipfs file
	if isCar {
		// offline api after to ensure we do not reach out to the network for any reason
		api, err = api.WithOptions(options.Api.Offline(true))
		fmt.Printf("getCarOrResolve api %+v\n", api)
		fmt.Println("getCarOrResolve api err", err)
		if err != nil {
			return nil, err
		}

		sdsP, err := sds.NewDagParser(ctx, api.Dag(), nd.Blockstore, nd.Pinning).Import(f.(files.File), doPinRoots)
		fmt.Printf("getCarOrResolve sdsP %+v\n", sdsP)
		fmt.Println("getCarOrResolve sdsP err", err)
		if err != nil {
			return nil, err
		}

		fmt.Printf("getCarOrResolve sdsP (before) %+v\n", sdsP)

		// for folder + file match
		if len(p.Segments()) > 2 {
			c := make([]string, len(p.Segments())-2)
			copy(c, p.Segments()[2:])

			sdsP, err = path.NewPath(filepath.Join(sdsP.String(), strings.Join(c, "/")))
			if err != nil {
				return nil, err
			}

			fmt.Printf("getCarOrResolve sdsP (after) %+v\n", sdsP)
		}

		f, err = api.Unixfs().Get(ctx, sdsP)
		fmt.Println("unixfs get err", err)
		if err != nil {
			return nil, err
		}
		fmt.Printf("getCarOrResolve f (origin) %+v\n", f)
	}

	return f, nil
}
