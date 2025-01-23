package conn

import (
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

func Parse(dsn string) (Connector, error) {
	if strings.Contains(dsn, "redis") {
		opts, err := redis.ParseClusterURL(dsn)
		if err != nil {
			return nil, err
		}
		return NewRc(opts)
	}

	return nil, fmt.Errorf("connector not implemented")
}
