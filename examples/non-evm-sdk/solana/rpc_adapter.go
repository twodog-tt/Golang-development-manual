package solanatx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const maxRPCResponseBytes int64 = 2 << 20

// RPCAdapter talks to Solana's official HTTP JSON-RPC surface. The community
// SDK remains responsible for offline transaction construction and signing.
type RPCAdapter struct {
	endpoint *url.URL
	client   *http.Client
}

func NewRPCAdapter(endpoint string, client *http.Client) (*RPCAdapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("parse Solana RPC URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Solana RPC URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("Solana RPC URL must include a host")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &RPCAdapter{endpoint: parsed, client: client}, nil
}

type NodeReadiness struct {
	Ready       bool
	Health      string
	GenesisHash string
}

// ValidateGenesisHash fails closed when a caller configured an expected
// cluster identity. Passing an empty expected value disables identity pinning.
func (readiness NodeReadiness) ValidateGenesisHash(expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	if readiness.GenesisHash != expected {
		return fmt.Errorf("Solana genesis hash = %q, expected %q", readiness.GenesisHash, expected)
	}
	return nil
}

// Readiness checks node health and reads the genesis hash used as the stable
// cluster identity. A healthy response alone does not identify the cluster;
// callers should use ValidateGenesisHash when the target network is known.
func (adapter *RPCAdapter) Readiness(ctx context.Context) (NodeReadiness, error) {
	var health string
	if err := adapter.call(ctx, "getHealth", nil, &health); err != nil {
		return NodeReadiness{}, fmt.Errorf("get Solana health: %w", err)
	}
	var genesisHash string
	if err := adapter.call(ctx, "getGenesisHash", nil, &genesisHash); err != nil {
		return NodeReadiness{}, fmt.Errorf("get Solana genesis hash: %w", err)
	}
	if genesisHash == "" {
		return NodeReadiness{}, errors.New("Solana RPC returned an empty genesis hash")
	}
	return NodeReadiness{
		Ready:       health == "ok",
		Health:      health,
		GenesisHash: genesisHash,
	}, nil
}

type SubmitOptions struct {
	SkipPreflight       bool
	PreflightCommitment rpc.CommitmentType
	MaxRetries          *uint
	MinContextSlot      *uint64
}

// Submit relays already-signed base64 transaction bytes. A returned signature
// only means the RPC accepted the request; it is not confirmation.
func (adapter *RPCAdapter) Submit(
	ctx context.Context,
	signedTransactionBase64 string,
	options SubmitOptions,
) (solana.Signature, error) {
	signedTransactionBase64 = strings.TrimSpace(signedTransactionBase64)
	decoded, err := base64.StdEncoding.DecodeString(signedTransactionBase64)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("decode signed Solana transaction: %w", err)
	}
	if len(decoded) == 0 {
		return solana.Signature{}, errors.New("signed Solana transaction is required")
	}
	decoder := bin.NewBinDecoder(decoded)
	transaction, err := solana.TransactionFromDecoder(decoder)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("decode signed Solana transaction: %w", err)
	}
	if decoder.HasRemaining() {
		return solana.Signature{}, fmt.Errorf("decode signed Solana transaction: %d trailing bytes", decoder.Remaining())
	}
	if len(transaction.Signatures) == 0 {
		return solana.Signature{}, errors.New("signed Solana transaction has no signatures")
	}
	if err = transaction.VerifySignatures(); err != nil {
		return solana.Signature{}, fmt.Errorf("verify signed Solana transaction: %w", err)
	}
	expectedSignature := transaction.Signatures[0]
	if options.PreflightCommitment != "" && !isModernCommitment(options.PreflightCommitment) {
		return solana.Signature{}, fmt.Errorf("unsupported preflight commitment %q", options.PreflightCommitment)
	}

	config := map[string]any{
		"encoding":      "base64",
		"skipPreflight": options.SkipPreflight,
	}
	if options.PreflightCommitment != "" {
		config["preflightCommitment"] = options.PreflightCommitment
	}
	if options.MaxRetries != nil {
		config["maxRetries"] = *options.MaxRetries
	}
	if options.MinContextSlot != nil {
		config["minContextSlot"] = *options.MinContextSlot
	}

	var encodedSignature string
	if err = adapter.call(
		ctx,
		"sendTransaction",
		[]any{signedTransactionBase64, config},
		&encodedSignature,
	); err != nil {
		return solana.Signature{}, fmt.Errorf("send Solana transaction: %w", err)
	}
	signature, err := solana.SignatureFromBase58(encodedSignature)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("Solana RPC returned invalid signature %q: %w", encodedSignature, err)
	}
	if signature != expectedSignature {
		return solana.Signature{}, fmt.Errorf(
			"Solana RPC returned signature %s, signed transaction ID is %s",
			signature,
			expectedSignature,
		)
	}
	return signature, nil
}

type StatusOptions struct {
	// RequiredCommitment defaults to confirmed. It controls when an observed,
	// successful signature becomes OutcomeSucceeded.
	RequiredCommitment rpc.ConfirmationStatusType
	// RecentBlockhash is optional. When the signature is not observed, the
	// adapter checks this blockhash separately and can return OutcomeExpired.
	RecentBlockhash *solana.Hash
	// BlockhashCommitment defaults to processed and is intentionally separate
	// from RequiredCommitment.
	BlockhashCommitment rpc.CommitmentType
}

type TransactionStatus struct {
	Signature          solana.Signature
	Outcome            Outcome
	Slot               uint64
	ConfirmationStatus rpc.ConfirmationStatusType
	ExecutionError     json.RawMessage
	BlockhashValid     *bool
}

// TransactionStatus queries the signature status cache and full history. A
// missing signature is UNKNOWN without expiry evidence. With an explicitly
// supplied recent blockhash it is PENDING while that hash remains valid and
// EXPIRED after validity is lost. An observed result remains pending until
// RequiredCommitment is reached.
func (adapter *RPCAdapter) TransactionStatus(
	ctx context.Context,
	signature solana.Signature,
	options StatusOptions,
) (TransactionStatus, error) {
	if signature == (solana.Signature{}) {
		return TransactionStatus{}, errors.New("Solana transaction signature is required")
	}
	required := options.RequiredCommitment
	if required == "" {
		required = rpc.ConfirmationStatusConfirmed
	}
	if !isConfirmationStatus(required) {
		return TransactionStatus{}, fmt.Errorf("unsupported required confirmation status %q", required)
	}

	var result struct {
		Value []json.RawMessage `json:"value"`
	}
	if err := adapter.call(
		ctx,
		"getSignatureStatuses",
		[]any{[]string{signature.String()}, map[string]any{"searchTransactionHistory": true}},
		&result,
	); err != nil {
		return TransactionStatus{}, fmt.Errorf("get Solana signature status: %w", err)
	}
	if len(result.Value) != 1 {
		return TransactionStatus{}, fmt.Errorf("Solana RPC returned %d signature statuses, expected 1", len(result.Value))
	}

	status := TransactionStatus{Signature: signature, Outcome: OutcomeUnknown}
	if isJSONNull(result.Value[0]) {
		if options.RecentBlockhash == nil {
			return status, nil
		}
		commitment := options.BlockhashCommitment
		if commitment == "" {
			commitment = rpc.CommitmentProcessed
		}
		if !isModernCommitment(commitment) {
			return TransactionStatus{}, fmt.Errorf("unsupported blockhash commitment %q", commitment)
		}
		var validity struct {
			Value bool `json:"value"`
		}
		if err := adapter.call(
			ctx,
			"isBlockhashValid",
			[]any{options.RecentBlockhash.String(), map[string]any{"commitment": commitment}},
			&validity,
		); err != nil {
			return TransactionStatus{}, fmt.Errorf("check Solana recent blockhash: %w", err)
		}
		valid := validity.Value
		status.BlockhashValid = &valid
		if valid {
			status.Outcome = OutcomePending
		} else {
			status.Outcome = OutcomeExpired
		}
		return status, nil
	}

	var observed struct {
		Slot               uint64                     `json:"slot"`
		Err                json.RawMessage            `json:"err"`
		ConfirmationStatus rpc.ConfirmationStatusType `json:"confirmationStatus"`
	}
	if err := json.Unmarshal(result.Value[0], &observed); err != nil {
		return TransactionStatus{}, fmt.Errorf("decode Solana signature status: %w", err)
	}
	status.Slot = observed.Slot
	status.ConfirmationStatus = observed.ConfirmationStatus
	var executionError any
	if !isJSONNull(observed.Err) {
		executionError = observed.Err
		status.ExecutionError = append(json.RawMessage(nil), observed.Err...)
	}
	status.Outcome = EvaluateStatus(&rpc.SignatureStatusesResult{
		Slot:               observed.Slot,
		Err:                executionError,
		ConfirmationStatus: observed.ConfirmationStatus,
	}, required)
	return status, nil
}

func isConfirmationStatus(status rpc.ConfirmationStatusType) bool {
	switch status {
	case rpc.ConfirmationStatusProcessed, rpc.ConfirmationStatusConfirmed, rpc.ConfirmationStatusFinalized:
		return true
	default:
		return false
	}
}

func isModernCommitment(commitment rpc.CommitmentType) bool {
	switch commitment {
	case rpc.CommitmentProcessed, rpc.CommitmentConfirmed, rpc.CommitmentFinalized:
		return true
	default:
		return false
	}
}

func isJSONNull(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *RPCError) Error() string {
	if len(bytes.TrimSpace(err.Data)) == 0 || isJSONNull(err.Data) {
		return fmt.Sprintf("Solana JSON-RPC error %d: %s", err.Code, err.Message)
	}
	return fmt.Sprintf("Solana JSON-RPC error %d: %s (%s)", err.Code, err.Message, bytes.TrimSpace(err.Data))
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (err *HTTPError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("Solana RPC HTTP status %d", err.StatusCode)
	}
	return fmt.Sprintf("Solana RPC HTTP status %d: %s", err.StatusCode, err.Body)
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
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("encode JSON-RPC request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, adapter.endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create JSON-RPC request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	response, err := adapter.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRPCResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read JSON-RPC response: %w", err)
	}
	if int64(len(body)) > maxRPCResponseBytes {
		return errors.New("Solana RPC response exceeds size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	var envelope rpcResponse
	if err = json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode JSON-RPC response: %w", err)
	}
	if envelope.JSONRPC != "" && envelope.JSONRPC != "2.0" {
		return fmt.Errorf("unexpected JSON-RPC version %q", envelope.JSONRPC)
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if isJSONNull(envelope.Result) {
		return errors.New("Solana RPC response is missing result")
	}
	if err = json.Unmarshal(envelope.Result, output); err != nil {
		return fmt.Errorf("decode JSON-RPC result: %w", err)
	}
	return nil
}
