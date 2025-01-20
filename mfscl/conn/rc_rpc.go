package conn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var _ Connector = (*rcRpc)(nil)

type rcReq struct {
	DataKey   string `json:"data_key"`
	DataValue string `json:"data_value,omitempty"`
}

type rcRes []string

func wrapRcReq(key string, val string) []byte {
	// compose json-rpc request
	request := &rcReq{
		DataKey:   key,
		DataValue: val,
	}
	r, e := json.Marshal(request)
	if e != nil {
		logger.Error("json marshal error", e)
		return nil
	}
	return r
}

var timeout = 10 * time.Second

type rcRpc struct {
	httpRpcUrl string
}

func NewRcRpc(httpRpcUrl string) (*rcRpc, error) {
	return &rcRpc{
		httpRpcUrl: httpRpcUrl,
	}, nil
}

func (rpc *rcRpc) sendRequest(path string, key string, val string) (string, error) {
	// wrap to the json-rpc message
	request := wrapRcReq(key, val)

	if len(request) < 300 {
		logger.Debug("--> ", string(request))
	} else {
		logger.Debug("--> ", string(request[:230]), "... \"}]}")
	}

	url := strings.TrimRight(rpc.httpRpcUrl, "/") + "/" + strings.TrimLeft(path, "/")

	// http post
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(request))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) < 300 {
		logger.Debug("<-- ", string(body))
	} else {
		logger.Debug("<-- ", string(body[:230]), "... \"}]}")
	}

	resp.Body.Close()

	if len(body) == 0 {
		logger.Error("emptry body after read buffer")
		return "", fmt.Errorf("empty response body")
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("failed to get/put key")
	}

	// handle rsp
	var rawJsonStr string
	err = json.Unmarshal([]byte(body), &rawJsonStr)
	if err != nil {
		return "", nil
	}

	var result rcRes
	err = json.Unmarshal([]byte(rawJsonStr), &result)
	if err != nil {
		return "", nil
	}

	return result[0], nil
}

func (rpc *rcRpc) Key() any {
	return "local:filesroot"
}

func (rpc *rcRpc) Get(_ context.Context, key any) ([]byte, error) {
	value, err := rpc.sendRequest("get_key_value", key.(string), "")
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func (rpc *rcRpc) Put(_ context.Context, key any, value []byte) error {
	_, err := rpc.sendRequest("set_key_value", key.(string), string(value))
	if err != nil {
		return err
	}
	return nil
}

func (rpc *rcRpc) Sync(ctx context.Context, prefix any) error {
	return nil
}
