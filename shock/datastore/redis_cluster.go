package datastore

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	ds "github.com/ipfs/go-datastore"
	query "github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
)

var (
	_ ds.Datastore = (*redisClusterDatastore)(nil)

	logger = logging.Logger("shock/datastore")
)

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
	res := strings.Join(key.List(), ":")
	logger.Debugf("RC key converted: %s", res)
	return res
}

func (d *redisClusterDatastore) Close() error {
	logger.Debugf("RC close")
	return d.rdb.Close()
}

func (d *redisClusterDatastore) Delete(ctx context.Context, key ds.Key) error {
	logger.Debugf("RC delet: %s", key)
	_, err := d.rdb.Del(ctx, d.convKey(key)).Result()
	if err != nil {
		return err
	}
	return nil
}

func (d *redisClusterDatastore) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	logger.Debugf("RC get: %s", key)
	value, err := d.rdb.Get(ctx, d.convKey(key)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ds.ErrNotFound
		}
		return nil, err
	}
	return value, nil
}

func (d *redisClusterDatastore) GetSize(ctx context.Context, key ds.Key) (int, error) {
	logger.Debugf("RC get size: %s", key)
	r, err := d.Get(ctx, key)
	if err != nil {
		return -1, err
	}
	return len(r), nil
}

func (d *redisClusterDatastore) Has(ctx context.Context, key ds.Key) (bool, error) {
	logger.Debugf("RC exists: %s", key)
	res, err := d.rdb.Exists(ctx, d.convKey(key)).Result()

	if err != nil {
		if err == redis.Nil {
			return false, ds.ErrNotFound
		}
		return false, err
	}
	return res == 1, nil
}

func (d *redisClusterDatastore) Put(ctx context.Context, key ds.Key, value []byte) error {
	logger.Debugf("RC put: %s - len: %d", key, len(value))
	_, err := d.rdb.Set(ctx, d.convKey(key), value, 0).Result()
	if err != nil {
		return err
	}
	return nil
}

func (d *redisClusterDatastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	logger.Debugf("RC query: %s", q)
	return nil, fmt.Errorf("not implemented")
}

func (d *redisClusterDatastore) Sync(ctx context.Context, prefix ds.Key) error {
	logger.Debugf("RC sync: %s", prefix)
	return nil
}
