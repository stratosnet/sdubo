package conn

import (
	"fmt"
	"strings"

	"github.com/ipfs/kubo/misc/sutil"
)

func Parse(dsn string) (Connector, error) {
	if strings.Contains(dsn, "http") {
		rpcUrl, err := sutil.ParseHTTPAddress(dsn)
		if err != nil {
			return nil, err
		}
		return NewRcRpc(rpcUrl)
	}

	return nil, fmt.Errorf("connector not implemented")
}
