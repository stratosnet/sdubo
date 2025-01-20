package mfscl

import (
	"context"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/mfscl/conn"
)

var logger = logging.Logger("mfscl")

type MFSCluster struct {
	connector conn.Connector
}

func (cl *MFSCluster) Provide(connector conn.Connector) {
	cl.connector = connector
}

func (cl *MFSCluster) Get(ctx context.Context) ([]byte, error) {
	return cl.connector.Get(ctx, cl.connector.Key())
}

func (cl *MFSCluster) Put(ctx context.Context, value []byte) error {
	return cl.connector.Put(ctx, cl.connector.Key(), value)
}

func (cl *MFSCluster) Sync(ctx context.Context) error {
	return cl.connector.Sync(ctx, cl.connector.Key())
}
