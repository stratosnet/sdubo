package iface

import (
	"context"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
)

// SdsAPI specifies the interface to the sds layer.
type SdsAPI interface {
	// Link a path with sds, adds it to the blockstore,
	// and returns the key representing that node.
	Link(context.Context, path.ImmutablePath, string, ...options.UnixfsAddOption) (path.ImmutablePath, error)
}
