package conn

import (
	"context"
)

type Connector interface {
	Key() any
	Get(ctx context.Context, key any) ([]byte, error)
	Put(ctx context.Context, key any, value []byte) error
	Sync(ctx context.Context, prefix any) error
}
