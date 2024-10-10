package iface

import (
	"context"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/coreiface/options"
)

// SdsAPI specifies the interface to the sds layer.
type SdsAPI interface {
	// Add imports the data from the reader into sds store chunks
	Upload(context.Context, files.File, ...options.UnixfsAddOption) (string, error)
	// Link a path with sds, adds it to the blockstore,
	// and returns the key representing that node.
	Link(context.Context, cid.Cid, string, ...options.UnixfsAddOption) (path.ImmutablePath, error)
	// Parse file to get sds file hash
	Parse(context.Context, files.File) (path.ImmutablePath, error)
	// Get returns a read-only handle to a file tree referenced by a file hash
	//
	// Note that some implementations of this API may apply the specified context
	// to operations performed on the returned file
	Download(context.Context, path.Path) (files.File, error)
}
