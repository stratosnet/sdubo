package sg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getTestClient() *Client {
	client := NewClient("http://localhost:8000")
	return client
}

func TestSG_UpdateTraffic(t *testing.T) {
	client := getTestClient()

	req := &TrafficRequest{
		ProjectID: 1,
		Traffic:   10000000,
		Time:      time.Now(),
	}
	err := client.UpdateTraffic(context.TODO(), req)
	assert.NoError(t, err)
}
