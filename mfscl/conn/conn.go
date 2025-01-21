package conn

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ipfs/kubo/misc/sutil"
	"github.com/redis/go-redis/v9"
)

func Parse(dsn string) (Connector, error) {
	if strings.Contains(dsn, "http") {
		rpcUrl, err := sutil.ParseHTTPAddress(dsn)
		if err != nil {
			return nil, err
		}
		return NewRcRpc(rpcUrl)
	} else if strings.Contains(dsn, "redis") {
		opts, err := parseRedisClusterDSN(dsn)
		if err != nil {
			return nil, err
		}
		return NewRc(opts)
	}

	return nil, fmt.Errorf("connector not implemented")
}

// parseRedisClusterDSN is used to parse string like "redis://:password@host1:6379,host2:6379,host3:6379?db=0&timeout=5s"
func parseRedisClusterDSN(dsn string) (*redis.ClusterOptions, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	hosts := strings.Split(u.Host, ",")
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts found in DSN")
	}

	password, _ := u.User.Password()

	timeout := rcTimeout

	query := u.Query()
	if t := query.Get("timeout"); t != "" {
		if parsedTimeout, err := time.ParseDuration(t); err == nil {
			timeout = parsedTimeout
		} else {
			return nil, fmt.Errorf("invalid timeout value: %s", t)
		}
	}

	options := &redis.ClusterOptions{
		Addrs:    hosts,
		Password: password,
	}

	options.DialTimeout = timeout
	options.ReadTimeout = timeout
	options.WriteTimeout = timeout

	return options, nil
}
