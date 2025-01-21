package conn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRC_SimplePutAndGet(t *testing.T) {
	opts, err := parseRedisClusterDSN("redis://127.0.0.1:6379")
	if err != nil {
		panic(err)
	}
	rcluster, err := NewRc(opts)
	if err != nil {
		panic(err)
	}

	requestKey := generateRandomString(16)
	expectedResult := []byte{18, 32, 89, 184, 61, 176, 74, 172, 193, 30, 16, 215, 130, 92, 225, 10, 203, 158, 214, 245, 187, 163, 126, 230, 54, 12, 96, 124, 209, 59, 230, 244, 74, 108}

	// 1. Check key for empty
	_, err = rcluster.Get(context.TODO(), requestKey)
	assert.Error(t, err)

	// 2. Add value to key
	err = rcluster.Put(context.TODO(), requestKey, expectedResult)
	assert.NoError(t, err)

	// 3. Check if it is stored
	res, err := rcluster.Get(context.TODO(), requestKey)
	assert.Equal(t, expectedResult, res)
	assert.NoError(t, err)
}
