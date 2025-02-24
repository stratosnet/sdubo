package conn

import (
	"context"
	"strings"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/kubo/repo"
)

var _ Connector = (*dsWrapper)(nil)

type dsWrapper struct {
	ds repo.Datastore
}

func NewDsWrapper(ds repo.Datastore) *dsWrapper {
	return &dsWrapper{ds}
}

func (conn *dsWrapper) Namespace() string {
	return conn.CreateKey(
		"",
		"local",
		"filesroot",
	)
}

func (conn *dsWrapper) ApplyNamespace(key string) string {
	if key == "" {
		return conn.Namespace()
	}
	return conn.CreateKey(
		conn.Namespace(),
		key,
	)
}

func (conn *dsWrapper) CreateKey(keys ...string) string {
	return strings.Join(keys, "/")
}

func (conn *dsWrapper) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := conn.ds.Get(ctx, datastore.NewKey(conn.ApplyNamespace(key)))
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func (conn *dsWrapper) Put(ctx context.Context, key string, value []byte) error {
	err := conn.ds.Put(ctx, datastore.NewKey(conn.ApplyNamespace(key)), value)
	if err != nil {
		return err
	}
	return nil
}

func (conn *dsWrapper) Rm(ctx context.Context, key string) error {
	err := conn.ds.Delete(ctx, datastore.NewKey(conn.ApplyNamespace(key)))
	if err != nil {
		return err
	}
	return nil
}

func (conn *dsWrapper) Sync(ctx context.Context, prefix string) error {
	return conn.ds.Sync(ctx, datastore.NewKey(conn.ApplyNamespace(prefix)))
}
