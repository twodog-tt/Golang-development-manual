package aptostx

import (
	"bytes"
	"context"
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

	aptos "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/bcs"
)

const (
	aptosSignedTransactionBCS = "application/x.aptos.signed_transaction+bcs"
	maxRESTResponseBytes      = int64(2 << 20)
)

// RESTAdapter targets the Aptos fullnode REST API. The existing SDK-backed
// builder remains responsible for offline BCS construction and signing.
type RESTAdapter struct {
	baseURL *url.URL
	client  *http.Client
}

func NewRESTAdapter(endpoint string, client *http.Client) (*RESTAdapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("parse Aptos REST URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Aptos REST URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("Aptos REST URL must include a host")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &RESTAdapter{baseURL: parsed, client: client}, nil
}

type NodeReadiness struct {
	Ready                bool
	ChainID              uint8
	Epoch                uint64
	LedgerVersion        uint64
	OldestLedgerVersion  uint64
	LedgerTimestampUsecs uint64
	BlockHeight          uint64
	NodeRole             string
}

// ValidateChainID pins the endpoint to the intended Aptos network. Passing an
// empty expected value disables identity pinning.
func (readiness NodeReadiness) ValidateChainID(expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	parsed, err := strconv.ParseUint(expected, 10, 8)
	if err != nil {
		return fmt.Errorf("parse expected Aptos chain ID %q: %w", expected, err)
	}
	if readiness.ChainID != uint8(parsed) {
		return fmt.Errorf("Aptos chain ID = %d, expected %d", readiness.ChainID, parsed)
	}
	return nil
}

// Readiness reads the ledger information exposed at the configured /v1 base
// URL. The returned chain ID should be validated against trusted config.
func (adapter *RESTAdapter) Readiness(ctx context.Context) (NodeReadiness, error) {
	var result struct {
		ChainID             json.RawMessage `json:"chain_id"`
		Epoch               json.RawMessage `json:"epoch"`
		LedgerVersion       json.RawMessage `json:"ledger_version"`
		OldestLedgerVersion json.RawMessage `json:"oldest_ledger_version"`
		LedgerTimestamp     json.RawMessage `json:"ledger_timestamp"`
		BlockHeight         json.RawMessage `json:"block_height"`
		NodeRole            string          `json:"node_role"`
	}
	if err := adapter.doJSON(ctx, http.MethodGet, adapter.baseURL, "", nil, &result); err != nil {
		return NodeReadiness{}, fmt.Errorf("get Aptos ledger info: %w", err)
	}
	chainID, err := parseAptosUint(result.ChainID, 8, "chain_id", false)
	if err != nil {
		return NodeReadiness{}, err
	}
	epoch, err := parseAptosUint(result.Epoch, 64, "epoch", false)
	if err != nil {
		return NodeReadiness{}, err
	}
	ledgerVersion, err := parseAptosUint(result.LedgerVersion, 64, "ledger_version", false)
	if err != nil {
		return NodeReadiness{}, err
	}
	oldestLedgerVersion, err := parseAptosUint(result.OldestLedgerVersion, 64, "oldest_ledger_version", true)
	if err != nil {
		return NodeReadiness{}, err
	}
	ledgerTimestamp, err := parseAptosUint(result.LedgerTimestamp, 64, "ledger_timestamp", false)
	if err != nil {
		return NodeReadiness{}, err
	}
	blockHeight, err := parseAptosUint(result.BlockHeight, 64, "block_height", true)
	if err != nil {
		return NodeReadiness{}, err
	}
	return NodeReadiness{
		Ready:                chainID != 0,
		ChainID:              uint8(chainID),
		Epoch:                epoch,
		LedgerVersion:        ledgerVersion,
		OldestLedgerVersion:  oldestLedgerVersion,
		LedgerTimestampUsecs: ledgerTimestamp,
		BlockHeight:          blockHeight,
		NodeRole:             result.NodeRole,
	}, nil
}

type TxOutcome string

const (
	TxUnknown   TxOutcome = "UNKNOWN"
	TxPending   TxOutcome = "PENDING"
	TxSucceeded TxOutcome = "SUCCEEDED"
	TxFailed    TxOutcome = "FAILED"
)

type SubmitResult struct {
	Hash    string
	Outcome TxOutcome
}

// Submit relays an already-signed BCS transaction. HTTP acceptance represents
// mempool admission only, so the result is always PENDING until queried.
func (adapter *RESTAdapter) Submit(ctx context.Context, signedBCS []byte) (SubmitResult, error) {
	if len(signedBCS) == 0 {
		return SubmitResult{}, errors.New("signed Aptos BCS transaction is required")
	}
	signedTransaction := &aptos.SignedTransaction{}
	if err := bcs.Deserialize(signedTransaction, signedBCS); err != nil {
		return SubmitResult{}, fmt.Errorf("decode signed Aptos BCS transaction: %w", err)
	}
	if err := signedTransaction.Verify(); err != nil {
		return SubmitResult{}, fmt.Errorf("verify signed Aptos transaction: %w", err)
	}
	expectedHash := aptosSignedTransactionHash(signedBCS)
	var result struct {
		Type string `json:"type"`
		Hash string `json:"hash"`
	}
	if err := adapter.doJSON(
		ctx,
		http.MethodPost,
		adapter.baseURL.JoinPath("transactions"),
		aptosSignedTransactionBCS,
		signedBCS,
		&result,
	); err != nil {
		return SubmitResult{}, fmt.Errorf("submit Aptos transaction: %w", err)
	}
	if result.Type != "" && result.Type != "pending_transaction" {
		return SubmitResult{}, fmt.Errorf("Aptos submit returned unexpected transaction type %q", result.Type)
	}
	hash, err := normalizeAptosHash(result.Hash)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("Aptos submit returned invalid hash: %w", err)
	}
	if hash != expectedHash {
		return SubmitResult{}, fmt.Errorf("Aptos submit returned hash %s, signed transaction hash is %s", hash, expectedHash)
	}
	return SubmitResult{Hash: hash, Outcome: TxPending}, nil
}

func aptosSignedTransactionHash(signedBCS []byte) string {
	prefix := aptos.Sha3256Hash([][]byte{[]byte("APTOS::Transaction")})
	hash := aptos.Sha3256Hash([][]byte{
		prefix,
		{byte(aptos.UserTransactionVariant)},
		signedBCS,
	})
	return aptos.BytesToHex(hash)
}

type TransactionStatus struct {
	Hash     string
	Type     string
	Outcome  TxOutcome
	Version  *uint64
	Success  *bool
	VMStatus string
}

// QueryTransaction preserves Aptos's distinct evidence states: a
// pending_transaction has no execution result; a committed transaction uses
// success=true for success and success=false plus vm_status for VM failure. A
// specific 404 transaction_not_found is UNKNOWN, not proof of pending or
// expiration; other REST failures remain structured errors.
func (adapter *RESTAdapter) QueryTransaction(ctx context.Context, hash string) (TransactionStatus, error) {
	normalizedHash, err := normalizeAptosHash(hash)
	if err != nil {
		return TransactionStatus{}, err
	}
	var result struct {
		Type     string          `json:"type"`
		Hash     string          `json:"hash"`
		Version  json.RawMessage `json:"version"`
		Success  *bool           `json:"success"`
		VMStatus string          `json:"vm_status"`
	}
	if err = adapter.doJSON(
		ctx,
		http.MethodGet,
		adapter.baseURL.JoinPath("transactions/by_hash", normalizedHash),
		"",
		nil,
		&result,
	); err != nil {
		var restErr *RESTError
		if errors.As(err, &restErr) && restErr.StatusCode == http.StatusNotFound && restErr.ErrorCode == "transaction_not_found" {
			return TransactionStatus{Hash: normalizedHash, Outcome: TxUnknown}, nil
		}
		return TransactionStatus{}, fmt.Errorf("query Aptos transaction: %w", err)
	}
	responseHash, err := normalizeAptosHash(result.Hash)
	if err != nil {
		return TransactionStatus{}, fmt.Errorf("Aptos query returned invalid hash: %w", err)
	}
	if responseHash != normalizedHash {
		return TransactionStatus{}, fmt.Errorf("Aptos returned transaction hash %s for requested %s", responseHash, normalizedHash)
	}
	status := TransactionStatus{
		Hash:     normalizedHash,
		Type:     result.Type,
		Success:  result.Success,
		VMStatus: result.VMStatus,
	}
	if result.Type == "pending_transaction" {
		status.Outcome = TxPending
		return status, nil
	}
	if result.Success == nil {
		return TransactionStatus{}, fmt.Errorf("committed Aptos transaction type %q is missing success", result.Type)
	}
	version, err := parseAptosUint(result.Version, 64, "transaction version", false)
	if err != nil {
		return TransactionStatus{}, err
	}
	status.Version = &version
	if *result.Success {
		status.Outcome = TxSucceeded
	} else {
		status.Outcome = TxFailed
	}
	return status, nil
}

func normalizeAptosHash(value string) (string, error) {
	value = strings.TrimSpace(value)
	withoutPrefix := strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X")
	decoded, err := hex.DecodeString(withoutPrefix)
	if err != nil {
		return "", fmt.Errorf("decode transaction hash: %w", err)
	}
	if len(decoded) != 32 {
		return "", fmt.Errorf("transaction hash is %d bytes, expected 32", len(decoded))
	}
	return "0x" + strings.ToLower(withoutPrefix), nil
}

func parseAptosUint(raw json.RawMessage, bitSize int, field string, emptyIsZero bool) (uint64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		if emptyIsZero {
			return 0, nil
		}
		return 0, fmt.Errorf("Aptos response is missing %s", field)
	}
	if strings.HasPrefix(value, `"`) {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return 0, fmt.Errorf("decode Aptos %s: %w", field, err)
		}
		value = decoded
	}
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, fmt.Errorf("decode Aptos %s %q: %w", field, value, err)
	}
	return parsed, nil
}

type RESTError struct {
	StatusCode  int
	ErrorCode   string
	Message     string
	VMErrorCode string
	Body        string
}

func (err *RESTError) Error() string {
	detail := err.Message
	if detail == "" {
		detail = err.Body
	}
	if err.ErrorCode != "" {
		detail = strings.TrimSpace(err.ErrorCode + ": " + detail)
	}
	if detail == "" {
		return fmt.Sprintf("Aptos REST HTTP status %d", err.StatusCode)
	}
	return fmt.Sprintf("Aptos REST HTTP status %d: %s", err.StatusCode, detail)
}

func (adapter *RESTAdapter) doJSON(
	ctx context.Context,
	method string,
	requestURL *url.URL,
	contentType string,
	body []byte,
	output any,
) error {
	request, err := http.NewRequestWithContext(ctx, method, requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Aptos REST request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxRESTResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Aptos REST response: %w", err)
	}
	if int64(len(responseBody)) > maxRESTResponseBytes {
		return errors.New("Aptos REST response exceeds size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return newRESTError(response.StatusCode, responseBody)
	}
	if output == nil {
		return nil
	}
	if err = json.Unmarshal(responseBody, output); err != nil {
		return fmt.Errorf("decode Aptos REST response: %w", err)
	}
	return nil
}

func newRESTError(statusCode int, body []byte) *RESTError {
	result := &RESTError{StatusCode: statusCode, Body: strings.TrimSpace(string(body))}
	var payload struct {
		ErrorCode   string          `json:"error_code"`
		Message     string          `json:"message"`
		VMErrorCode json.RawMessage `json:"vm_error_code"`
	}
	if json.Unmarshal(body, &payload) == nil {
		result.ErrorCode = payload.ErrorCode
		result.Message = payload.Message
		if len(payload.VMErrorCode) != 0 && string(payload.VMErrorCode) != "null" {
			result.VMErrorCode = strings.Trim(string(payload.VMErrorCode), `"`)
		}
	}
	return result
}
