package conn

import (
	"context"
)

type Connector interface {
	Namespace() string
	ApplyNamespace(key string) string
	CreateKey(keys ...string) string
	Get(ctx context.Context, key string) ([]byte, error)
	Rm(ctx context.Context, key string) error
	Put(ctx context.Context, key string, value []byte) error
	Sync(ctx context.Context, prefix string) error
}
