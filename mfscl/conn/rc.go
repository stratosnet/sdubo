package conn

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var _ Connector = (*rc)(nil)

type rc struct {
	rdb *redis.ClusterClient
}

func NewRc(opts *redis.ClusterOptions) (*rc, error) {
	rdb := redis.NewClusterClient(opts)

	err := rdb.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return nil, err
	}
	r := &rc{
		rdb: rdb,
	}
	return r, nil
}

func (conn *rc) Namespace() string {
	return "mfs:local:filesroot"
}

func (conn *rc) CreateKey(key string) string {
	if key == "" {
		return conn.Namespace()
	}
	return conn.Namespace() + ":" + key
}

func (conn *rc) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := conn.rdb.Get(ctx, conn.CreateKey(key)).Result()
	switch {
	case err != nil:
		return nil, err
	case value == "":
		return nil, nil
	}
	return []byte(value), nil
}

func (conn *rc) Put(ctx context.Context, key string, value []byte) error {
	_, err := conn.rdb.Set(ctx, conn.CreateKey(key), value, 0).Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *rc) Sync(ctx context.Context, prefix string) error {
	return nil
}
