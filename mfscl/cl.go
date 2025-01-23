package mfscl

import (
	"context"

	"github.com/ipfs/boxo/mfs"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/mfscl/conn"
)

var _ = logging.Logger("mfscl")

type GetRoot func(ns string) (*mfs.Root, error)

type MFSCluster struct {
	connector conn.Connector
}

func (cl *MFSCluster) Provide(connector conn.Connector) {
	cl.connector = connector
}

func (cl *MFSCluster) Get(ctx context.Context, key string) ([]byte, error) {
	return cl.connector.Get(ctx, key)
}

func (cl *MFSCluster) Put(ctx context.Context, key string, value []byte) error {
	return cl.connector.Put(ctx, key, value)
}

func (cl *MFSCluster) Sync(ctx context.Context, key string) error {
	return cl.connector.Sync(ctx, key)
}
