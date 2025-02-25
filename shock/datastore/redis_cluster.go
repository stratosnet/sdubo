package datastore

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	ds "github.com/ipfs/go-datastore"
	query "github.com/ipfs/go-datastore/query"
)

var _ ds.Datastore = (*redisClusterDatastore)(nil)

type redisClusterDatastore struct {
	rdb *redis.ClusterClient
}

func NewRedisDatastore(opts *redis.ClusterOptions) (ds.Datastore, error) {
	rdb := redis.NewClusterClient(opts)

	err := rdb.ForEachShard(context.Background(), func(ctx context.Context, shard *redis.Client) error {
		return shard.Ping(ctx).Err()
	})
	if err != nil {
		return nil, err
	}
	d := &redisClusterDatastore{
		rdb: rdb,
	}
	return d, nil
}

func (d *redisClusterDatastore) convKey(key ds.Key) string {
	return strings.Join(key.List(), ":")
}

func (d *redisClusterDatastore) Close() error {
	return d.rdb.Close()
}

func (d *redisClusterDatastore) Delete(ctx context.Context, key ds.Key) error {
	_, err := d.rdb.Del(ctx, d.convKey(key)).Result()
	if err != nil {
		return err
	}
	return nil
}

func (d *redisClusterDatastore) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	value, err := d.rdb.Get(ctx, d.convKey(key)).Result()
	switch {
	case err == redis.Nil:
		return nil, nil
	case err != nil:
		return nil, err
	case value == "":
		return nil, nil
	}
	return []byte(value), nil
}

func (d *redisClusterDatastore) GetSize(ctx context.Context, key ds.Key) (int, error) {
	r, err := d.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	return len(r), nil
}

func (d *redisClusterDatastore) Has(ctx context.Context, key ds.Key) (bool, error) {
	res, err := d.rdb.Exists(ctx, d.convKey(key)).Result()
	switch {
	case err == redis.Nil:
		return false, nil
	case err != nil:
		return false, err
	case res == 0:
		return false, nil
	}
	return true, nil
}

func (d *redisClusterDatastore) Put(ctx context.Context, key ds.Key, value []byte) error {
	_, err := d.rdb.Set(ctx, d.convKey(key), value, 0).Result()
	if err != nil {
		return err
	}
	return nil
}

func (d *redisClusterDatastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	return nil, fmt.Errorf("not implemented")
}

func (d *redisClusterDatastore) Sync(ctx context.Context, prefix ds.Key) error {
	return nil
}
