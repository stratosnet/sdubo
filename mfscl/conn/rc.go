package conn

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type rc struct {
	rdb *redis.ClusterClient
}

var rcTimeout = 5 * time.Second // default timeout

func NewRc(opts *redis.ClusterOptions) (*rc, error) {
	rdb := redis.NewClusterClient(opts)

	err := rdb.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return nil, err
	}
	return &rc{
		rdb: rdb,
	}, nil
}

func (rc *rc) Key() any {
	return "local:filesroot"
}

func (r *rc) Get(ctx context.Context, key any) ([]byte, error) {
	value, err := r.rdb.Get(ctx, key.(string)).Result()
	switch {
	case err != nil:
		return nil, err
	case value == "":
		return nil, nil
	}
	return []byte(value), nil
}

func (r *rc) Put(ctx context.Context, key any, value []byte) error {
	_, err := r.rdb.Set(ctx, key.(string), value, 0).Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *rc) Sync(ctx context.Context, prefix any) error {
	return nil
}
