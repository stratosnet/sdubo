package sg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var rpcLogger = logging.Logger("sg/api")

var timeout = 30 * time.Second

type Client struct {
	BaseURI string
}

func NewClient(baseURI string) *Client {
	client := &Client{
		BaseURI: baseURI,
	}
	return client
}

func (c *Client) makeRequest(ctx context.Context, request Request) ([]byte, error) {
	var body io.Reader
	if request.Json != nil {
		if len(request.Json) < 300 {
			rpcLogger.Debug("pdj client", "--> ", string(request.Json))
		} else {
			rpcLogger.Debug("pdj client", "--> ", fmt.Sprintf("%s...\"}]}", request.Json[:230]))
		}
		body = bytes.NewBuffer(request.Json)
	}

	fullURL := c.BaseURI + request.Path

	if request.Params != nil {
		u, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		q := u.Query()
		for k, v := range request.Params.ToParams() {
			q.Set(k, fmt.Sprintf("%v", v))
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}

	req, err := http.NewRequestWithContext(ctx, request.Method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if request.Headers != nil {
		for k, v := range request.Headers {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	if len(respBody) < 300 {
		rpcLogger.Debug("pdj client", "<-- ", string(respBody))
	} else {
		rpcLogger.Debug("pdj client", "<-- ", fmt.Sprintf("%s...\"}]}", respBody[:230]))
	}
	return respBody, nil
}

func (c *Client) UpdateTraffic(ctx context.Context, req *TrafficRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if _, err = c.makeRequest(ctx, Request{
		Path:   fmt.Sprintf("/api/traffic/%d", req.ProjectID),
		Method: "POST",
		Json:   payload,
	}); err != nil {
		return err
	}

	return nil
}
