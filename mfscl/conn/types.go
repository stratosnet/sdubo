package conn

import (
	"context"
)

type Connector interface {
	Namespace() string
	CreateKey(key string) string
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Sync(ctx context.Context, prefix string) error
}
