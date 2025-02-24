package conn

import (
	"context"
	"strings"

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
	return conn.CreateKey(
		"mfs",
		"local",
		"filesroot",
	)
}

func (conn *rc) ApplyNamespace(key string) string {
	if key == "" {
		return conn.Namespace()
	}
	return conn.CreateKey(
		conn.Namespace(),
		key,
	)
}

func (conn *rc) CreateKey(keys ...string) string {
	return strings.Join(keys, ":")
}

func (conn *rc) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := conn.rdb.Get(ctx, conn.ApplyNamespace(key)).Result()
	switch {
	case err != nil:
		return nil, err
	case value == "":
		return nil, nil
	}
	return []byte(value), nil
}

func (conn *rc) Put(ctx context.Context, key string, value []byte) error {
	_, err := conn.rdb.Set(ctx, conn.ApplyNamespace(key), value, 0).Result()
	if err != nil {
		return err
	}
	return nil
}

func (conn *rc) Rm(ctx context.Context, key string) error {
	_, err := conn.rdb.Del(ctx, conn.ApplyNamespace(key)).Result()
	if err != nil {
		return err
	}
	return nil
}

func (r *rc) Sync(ctx context.Context, prefix string) error {
	return nil
}
