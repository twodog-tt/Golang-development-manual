// Package ethrpc 演示最小以太坊 JSON-RPC 客户端（S-BC-02 示例，无 go-ethereum 依赖）。
package ethrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client 调用 JSON-RPC 2.0 节点。
type Client struct {
	URL    string
	HTTP   *http.Client
	id     int
}

func New(url string) *Client {
	return &Client{URL: url, HTTP: http.DefaultClient}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	c.id++
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.id,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var out rpcResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}

// BlockNumber 调用 eth_blockNumber。
func (c *Client) BlockNumber(ctx context.Context) (string, error) {
	result, err := c.call(ctx, "eth_blockNumber")
	if err != nil {
		return "", err
	}
	var hex string
	if err := json.Unmarshal(result, &hex); err != nil {
		return "", err
	}
	return hex, nil
}

// GetTransactionReceipt 调用 eth_getTransactionReceipt。
func (c *Client) GetTransactionReceipt(ctx context.Context, txHash string) (json.RawMessage, error) {
	return c.call(ctx, "eth_getTransactionReceipt", txHash)
}
