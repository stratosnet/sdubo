package conn

import (
	"context"
	"math/rand"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	randomString := ""
	for i := 0; i < length; i++ {
		randomString += string(charset[rand.Intn(len(charset))])
	}
	return randomString
}

func TestRC_SimplePutAndGet(t *testing.T) {
	opts, err := redis.ParseClusterURL("redis://127.0.0.1:6379")
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
