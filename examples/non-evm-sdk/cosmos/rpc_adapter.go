package cosmostx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRPCResponseBytes int64 = 2 << 20

// RPCAdapter targets CometBFT's stable HTTP JSON-RPC interface. Cosmos SDK
// transaction construction and SIGN_MODE_DIRECT signing remain offline.
type RPCAdapter struct {
	endpoint *url.URL
	client   *http.Client
}

func NewRPCAdapter(endpoint string, client *http.Client) (*RPCAdapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("parse CometBFT RPC URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("CometBFT RPC URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("CometBFT RPC URL must include a host")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &RPCAdapter{endpoint: parsed, client: client}, nil
}

type NodeReadiness struct {
	Ready             bool
	ChainID           string
	NodeID            string
	Version           string
	LatestBlockHeight uint64
	CatchingUp        bool
}

// ValidateChainID prevents an HTTP-healthy endpoint for the wrong CometBFT
// network from being accepted. Passing an empty expected value disables the
// identity check.
func (readiness NodeReadiness) ValidateChainID(expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	if readiness.ChainID != expected {
		return fmt.Errorf("CometBFT chain ID = %q, expected %q", readiness.ChainID, expected)
	}
	return nil
}

// Readiness gets sync status and the node_info.network chain identity in one
// stable CometBFT status call.
func (adapter *RPCAdapter) Readiness(ctx context.Context) (NodeReadiness, error) {
	var result struct {
		NodeInfo struct {
			ID      string `json:"id"`
			Network string `json:"network"`
			Version string `json:"version"`
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockHeight json.RawMessage `json:"latest_block_height"`
			CatchingUp        bool            `json:"catching_up"`
		} `json:"sync_info"`
	}
	if err := adapter.call(ctx, "status", nil, &result); err != nil {
		return NodeReadiness{}, fmt.Errorf("get CometBFT status: %w", err)
	}
	if result.NodeInfo.Network == "" {
		return NodeReadiness{}, errors.New("CometBFT status returned an empty chain ID")
	}
	height, err := parseUint(result.SyncInfo.LatestBlockHeight, 64, "latest_block_height", false)
	if err != nil {
		return NodeReadiness{}, err
	}
	return NodeReadiness{
		Ready:             !result.SyncInfo.CatchingUp,
		ChainID:           result.NodeInfo.Network,
		NodeID:            result.NodeInfo.ID,
		Version:           result.NodeInfo.Version,
		LatestBlockHeight: height,
		CatchingUp:        result.SyncInfo.CatchingUp,
	}, nil
}

type TxOutcome string

const (
	TxUnknown   TxOutcome = "UNKNOWN"
	TxPending   TxOutcome = "PENDING"
	TxRejected  TxOutcome = "REJECTED"
	TxSucceeded TxOutcome = "SUCCEEDED"
	TxFailed    TxOutcome = "FAILED"
)

type BroadcastResult struct {
	Hash        string
	Outcome     TxOutcome
	CheckTxCode uint32
	Codespace   string
	Log         string
}

// BroadcastSync runs CometBFT broadcast_tx_sync. Code zero only means CheckTx
// accepted the transaction into this node's mempool, so the outcome is PENDING.
// A non-zero CheckTx code is REJECTED, not a committed execution failure.
// QueryTransaction is required for SUCCEEDED or FAILED.
func (adapter *RPCAdapter) BroadcastSync(ctx context.Context, txBytes []byte) (BroadcastResult, error) {
	if len(txBytes) == 0 {
		return BroadcastResult{}, errors.New("signed Cosmos transaction bytes are required")
	}
	var result struct {
		Code      json.RawMessage `json:"code"`
		Log       string          `json:"log"`
		Codespace string          `json:"codespace"`
		Hash      string          `json:"hash"`
	}
	if err := adapter.call(
		ctx,
		"broadcast_tx_sync",
		map[string]any{"tx": base64.StdEncoding.EncodeToString(txBytes)},
		&result,
	); err != nil {
		return BroadcastResult{}, fmt.Errorf("broadcast Cosmos transaction: %w", err)
	}
	codeValue, err := parseUint(result.Code, 32, "CheckTx code", true)
	if err != nil {
		return BroadcastResult{}, err
	}
	hash, _, err := normalizeCometHash(result.Hash)
	if err != nil {
		return BroadcastResult{}, fmt.Errorf("CometBFT returned invalid transaction hash: %w", err)
	}
	expectedHashBytes := sha256.Sum256(txBytes)
	expectedHash := strings.ToUpper(hex.EncodeToString(expectedHashBytes[:]))
	if hash != expectedHash {
		return BroadcastResult{}, fmt.Errorf("CometBFT returned hash %s, signed transaction hash is %s", hash, expectedHash)
	}
	outcome := TxPending
	if codeValue != 0 {
		outcome = TxRejected
	}
	return BroadcastResult{
		Hash:        hash,
		Outcome:     outcome,
		CheckTxCode: uint32(codeValue),
		Codespace:   result.Codespace,
		Log:         result.Log,
	}, nil
}

type TransactionStatus struct {
	Hash      string
	Found     bool
	Outcome   TxOutcome
	Height    uint64
	Index     uint32
	Code      uint32
	Codespace string
	Log       string
}

// QueryTransaction uses CometBFT's transaction index. A recognized "not
// found" RPC error is UNKNOWN with Found=false because it proves neither
// mempool admission nor expiration. Indexing-disabled and provider errors
// remain errors instead of being hidden as UNKNOWN.
func (adapter *RPCAdapter) QueryTransaction(ctx context.Context, hash string) (TransactionStatus, error) {
	normalizedHash, hashBytes, err := normalizeCometHash(hash)
	if err != nil {
		return TransactionStatus{}, err
	}
	var result struct {
		Hash   string          `json:"hash"`
		Height json.RawMessage `json:"height"`
		Index  json.RawMessage `json:"index"`
		Result struct {
			Code      json.RawMessage `json:"code"`
			Codespace string          `json:"codespace"`
			Log       string          `json:"log"`
		} `json:"tx_result"`
	}
	err = adapter.call(ctx, "tx", map[string]any{
		"hash":  base64.StdEncoding.EncodeToString(hashBytes),
		"prove": false,
	}, &result)
	if err != nil {
		var rpcErr *RPCError
		if errors.As(err, &rpcErr) && rpcErr.IsNotFound() {
			return TransactionStatus{Hash: normalizedHash, Outcome: TxUnknown}, nil
		}
		return TransactionStatus{}, fmt.Errorf("query Cosmos transaction: %w", err)
	}
	if result.Hash != "" {
		responseHash, _, normalizeErr := normalizeCometHash(result.Hash)
		if normalizeErr != nil {
			return TransactionStatus{}, fmt.Errorf("CometBFT returned invalid transaction hash: %w", normalizeErr)
		}
		if responseHash != normalizedHash {
			return TransactionStatus{}, fmt.Errorf("CometBFT returned transaction hash %s for requested %s", responseHash, normalizedHash)
		}
	}
	height, err := parseUint(result.Height, 64, "transaction height", false)
	if err != nil {
		return TransactionStatus{}, err
	}
	index, err := parseUint(result.Index, 32, "transaction index", true)
	if err != nil {
		return TransactionStatus{}, err
	}
	codeValue, err := parseUint(result.Result.Code, 32, "ExecTx code", true)
	if err != nil {
		return TransactionStatus{}, err
	}
	outcome := TxSucceeded
	if codeValue != 0 {
		outcome = TxFailed
	}
	return TransactionStatus{
		Hash:      normalizedHash,
		Found:     true,
		Outcome:   outcome,
		Height:    height,
		Index:     uint32(index),
		Code:      uint32(codeValue),
		Codespace: result.Result.Codespace,
		Log:       result.Result.Log,
	}, nil
}

func normalizeCometHash(value string) (string, []byte, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return "", nil, fmt.Errorf("decode transaction hash: %w", err)
	}
	if len(decoded) != 32 {
		return "", nil, fmt.Errorf("transaction hash is %d bytes, expected 32", len(decoded))
	}
	return strings.ToUpper(value), decoded, nil
}

func parseUint(raw json.RawMessage, bitSize int, field string, emptyIsZero bool) (uint64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		if emptyIsZero {
			return 0, nil
		}
		return 0, fmt.Errorf("CometBFT response is missing %s", field)
	}
	if strings.HasPrefix(value, `"`) {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return 0, fmt.Errorf("decode %s: %w", field, err)
		}
		value = decoded
	}
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("decode %s %q: %w", field, value, err)
	}
	return parsed, nil
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *RPCError) Error() string {
	if len(bytes.TrimSpace(err.Data)) == 0 || isJSONNull(err.Data) {
		return fmt.Sprintf("CometBFT JSON-RPC error %d: %s", err.Code, err.Message)
	}
	return fmt.Sprintf("CometBFT JSON-RPC error %d: %s (%s)", err.Code, err.Message, bytes.TrimSpace(err.Data))
}

func (err *RPCError) IsNotFound() bool {
	text := strings.ToLower(err.Message + " " + string(err.Data))
	return strings.Contains(text, "not found") && !strings.Contains(text, "indexing is disabled")
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (err *HTTPError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("CometBFT RPC HTTP status %d", err.StatusCode)
	}
	return fmt.Sprintf("CometBFT RPC HTTP status %d: %s", err.StatusCode, err.Body)
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
}

func (adapter *RPCAdapter) call(ctx context.Context, method string, params any, output any) error {
	payload, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("encode CometBFT JSON-RPC request: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		adapter.endpoint.String(),
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create CometBFT JSON-RPC request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRPCResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read CometBFT JSON-RPC response: %w", err)
	}
	if int64(len(body)) > maxRPCResponseBytes {
		return errors.New("CometBFT RPC response exceeds size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	var envelope rpcResponse
	if err = json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode CometBFT JSON-RPC response: %w", err)
	}
	if envelope.JSONRPC != "" && envelope.JSONRPC != "2.0" {
		return fmt.Errorf("unexpected JSON-RPC version %q", envelope.JSONRPC)
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if isJSONNull(envelope.Result) {
		return errors.New("CometBFT RPC response is missing result")
	}
	if err = json.Unmarshal(envelope.Result, output); err != nil {
		return fmt.Errorf("decode CometBFT JSON-RPC result: %w", err)
	}
	return nil
}

func isJSONNull(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}
