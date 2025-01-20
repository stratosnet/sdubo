package conn

import (
	"context"

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

func (w *dsWrapper) Key() any {
	return datastore.NewKey("/local/filesroot")
}

func (w *dsWrapper) Get(ctx context.Context, key any) ([]byte, error) {
	value, err := w.ds.Get(ctx, key.(datastore.Key))
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func (w *dsWrapper) Put(ctx context.Context, key any, value []byte) error {
	err := w.ds.Put(ctx, key.(datastore.Key), value)
	if err != nil {
		return err
	}
	return nil
}

func (w *dsWrapper) Sync(ctx context.Context, prefix any) error {
	return w.ds.Sync(ctx, prefix.(datastore.Key))
}
