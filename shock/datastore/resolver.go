package datastore

import (
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	ds "github.com/ipfs/go-datastore"
)

func Parse(dsn string) (ds.Datastore, error) {
	if strings.Contains(dsn, "redis") {
		opts, err := redis.ParseClusterURL(dsn)
		if err != nil {
			return nil, err
		}
		return NewRedisDatastore(opts)
	}

	return nil, fmt.Errorf("connector not implemented")
}

func JoinKeys(keys ...ds.Key) ds.Key {
	var keyJ []string

	for _, key := range keys {
		keyJ = append(keyJ, key.List()...)
	}

	return ds.NewKey(strings.Join(keyJ, "/"))
}
