package iface

import (
	"context"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/kubo/sds"
)

// SdsAPI specifies the interface to the sds layer.
type SdsAPI interface {
	// Link a path with sds, adds it to the blockstore,
	// and returns the key representing that node.
	Link(context.Context, *sds.SdsLinker, ...options.UnixfsAddOption) (path.ImmutablePath, error)
	// Add imports the data from the reader into sds store chunks
	Add(context.Context, files.Node, ...options.UnixfsAddOption) (string, error)
	// Parse file to get sds file hash
	Parse(context.Context, files.Node) (*sds.SdsLinker, error)
	// Get returns a read-only handle to a file tree referenced by a file hash
	//
	// Note that some implementations of this API may apply the specified context
	// to operations performed on the returned file
	Get(context.Context, *sds.SdsLinker) (files.Node, error)
}
