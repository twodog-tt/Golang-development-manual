package suiadapter

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
	"strconv"
	"strings"
	"time"
)

const (
	maxGraphQLResponseBytes int64 = 2 << 20

	readinessQuery = `query Readiness {
  chainIdentifier
  checkpoint {
    sequenceNumber
    digest
  }
}`

	transactionQuery = `query Transaction($digest: String!) {
  transaction(digest: $digest) {
    digest
    effects {
      status
      executionError { message }
      transaction { digest }
      checkpoint { sequenceNumber }
    }
  }
}`

	// ExecutionResult in the current schema exposes effects only. Operation and
	// input errors are returned through the GraphQL top-level errors array.
	executeTransactionMutation = `mutation ExecuteTransaction($transactionDataBcs: Base64!, $signatures: [Base64!]!) {
  executeTransaction(transactionDataBcs: $transactionDataBcs, signatures: $signatures) {
    effects {
      status
      executionError { message }
      transaction { digest }
      checkpoint { sequenceNumber }
    }
  }
}`
)

// GraphQLAdapter uses Sui's current GraphQL query and mutation surface. It has
// no deprecated JSON-RPC fallback.
type GraphQLAdapter struct {
	endpoint *url.URL
	client   *http.Client
}

func NewGraphQLAdapter(endpoint string, client *http.Client) (*GraphQLAdapter, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return nil, fmt.Errorf("parse Sui GraphQL URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("Sui GraphQL URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("Sui GraphQL URL must include a host")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &GraphQLAdapter{endpoint: parsed, client: client}, nil
}

// EndpointSupport makes transport capabilities explicit instead of silently
// falling back to the deprecated Sui JSON-RPC API.
type EndpointSupport struct {
	Transport          string
	Readiness          bool
	QueryTransaction   bool
	ExecuteTransaction bool
	DeprecatedJSONRPC  bool
}

func (adapter *GraphQLAdapter) EndpointSupport() EndpointSupport {
	return EndpointSupport{
		Transport:          "GraphQL",
		Readiness:          true,
		QueryTransaction:   true,
		ExecuteTransaction: true,
		DeprecatedJSONRPC:  false,
	}
}

type NodeReadiness struct {
	Ready                    bool
	ChainIdentifier          string
	CheckpointSequenceNumber uint64
	CheckpointDigest         string
}

// ValidateChainIdentifier pins the endpoint to a trusted Sui chain identity.
// Passing an empty expected value disables identity pinning.
func (readiness NodeReadiness) ValidateChainIdentifier(expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	if readiness.ChainIdentifier != expected {
		return fmt.Errorf("Sui chain identifier = %q, expected %q", readiness.ChainIdentifier, expected)
	}
	return nil
}

// Readiness reads the chain identifier and latest indexed checkpoint.
func (adapter *GraphQLAdapter) Readiness(ctx context.Context) (NodeReadiness, error) {
	var result struct {
		ChainIdentifier string `json:"chainIdentifier"`
		Checkpoint      *struct {
			SequenceNumber json.RawMessage `json:"sequenceNumber"`
			Digest         string          `json:"digest"`
		} `json:"checkpoint"`
	}
	if err := adapter.post(ctx, "Readiness", readinessQuery, nil, &result); err != nil {
		return NodeReadiness{}, fmt.Errorf("query Sui readiness: %w", err)
	}
	if result.ChainIdentifier == "" {
		return NodeReadiness{}, errors.New("Sui GraphQL returned an empty chain identifier")
	}
	if result.Checkpoint == nil {
		return NodeReadiness{
			ChainIdentifier: result.ChainIdentifier,
		}, nil
	}
	sequence, err := parseGraphQLUint(result.Checkpoint.SequenceNumber, "checkpoint sequence number")
	if err != nil {
		return NodeReadiness{}, err
	}
	return NodeReadiness{
		Ready:                    true,
		ChainIdentifier:          result.ChainIdentifier,
		CheckpointSequenceNumber: sequence,
		CheckpointDigest:         result.Checkpoint.Digest,
	}, nil
}

type TxOutcome string

const (
	TxUnknown   TxOutcome = "UNKNOWN"
	TxPending   TxOutcome = "PENDING"
	TxSucceeded TxOutcome = "SUCCEEDED"
	TxFailed    TxOutcome = "FAILED"
)

type TransactionStatus struct {
	Digest                   string
	Found                    bool
	Outcome                  TxOutcome
	ExecutionError           string
	CheckpointSequenceNumber *uint64
}

// QueryTransaction uses the current Query.transaction mapping. A null indexed
// result is UNKNOWN with Found=false because index lag/not-found proves neither
// admission nor expiration.
func (adapter *GraphQLAdapter) QueryTransaction(ctx context.Context, digest string) (TransactionStatus, error) {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return TransactionStatus{}, errors.New("Sui transaction digest is required")
	}
	var result struct {
		Transaction *struct {
			Digest  string              `json:"digest"`
			Effects *transactionEffects `json:"effects"`
		} `json:"transaction"`
	}
	if err := adapter.post(
		ctx,
		"Transaction",
		transactionQuery,
		map[string]any{"digest": digest},
		&result,
	); err != nil {
		return TransactionStatus{}, fmt.Errorf("query Sui transaction: %w", err)
	}
	if result.Transaction == nil {
		return TransactionStatus{Digest: digest, Outcome: TxUnknown}, nil
	}
	if result.Transaction.Digest == "" {
		return TransactionStatus{}, errors.New("Sui transaction query returned an empty digest")
	}
	if result.Transaction.Digest != digest {
		return TransactionStatus{}, fmt.Errorf(
			"Sui returned transaction digest %s for requested %s",
			result.Transaction.Digest,
			digest,
		)
	}
	if result.Transaction.Effects == nil {
		return TransactionStatus{Digest: digest, Found: true, Outcome: TxPending}, nil
	}
	status, err := statusFromEffects(result.Transaction.Digest, result.Transaction.Effects)
	if err != nil {
		return TransactionStatus{}, err
	}
	status.Found = true
	return status, nil
}

// Submit executes signed TransactionData BCS through the current GraphQL
// Mutation.executeTransaction surface. The returned transaction ID is taken
// only from effects.transaction.digest; effects digests are not selected or
// treated as transaction IDs.
func (adapter *GraphQLAdapter) Submit(
	ctx context.Context,
	transactionDataBCS string,
	signatures []string,
) (TransactionStatus, error) {
	transactionDataBCS = strings.TrimSpace(transactionDataBCS)
	if err := validateBase64("transactionDataBcs", transactionDataBCS); err != nil {
		return TransactionStatus{}, err
	}
	if len(signatures) == 0 {
		return TransactionStatus{}, errors.New("at least one Sui transaction signature is required")
	}
	normalizedSignatures := make([]string, len(signatures))
	for index, signature := range signatures {
		signature = strings.TrimSpace(signature)
		if err := validateBase64(fmt.Sprintf("signatures[%d]", index), signature); err != nil {
			return TransactionStatus{}, err
		}
		normalizedSignatures[index] = signature
	}

	var result struct {
		ExecuteTransaction *struct {
			Effects *transactionEffects `json:"effects"`
		} `json:"executeTransaction"`
	}
	if err := adapter.post(
		ctx,
		"ExecuteTransaction",
		executeTransactionMutation,
		map[string]any{
			"transactionDataBcs": transactionDataBCS,
			"signatures":         normalizedSignatures,
		},
		&result,
	); err != nil {
		return TransactionStatus{}, fmt.Errorf("execute Sui transaction: %w", err)
	}
	if result.ExecuteTransaction == nil || result.ExecuteTransaction.Effects == nil {
		return TransactionStatus{}, errors.New("Sui executeTransaction returned no effects")
	}
	effects := result.ExecuteTransaction.Effects
	if effects.Transaction == nil || effects.Transaction.Digest == "" {
		return TransactionStatus{}, errors.New("Sui executeTransaction effects are missing transaction.digest")
	}
	status, err := statusFromEffects(effects.Transaction.Digest, effects)
	if err != nil {
		return TransactionStatus{}, err
	}
	if status.Outcome == TxPending {
		return TransactionStatus{}, errors.New("Sui executeTransaction effects are missing execution status")
	}
	status.Found = true
	return status, nil
}

type transactionEffects struct {
	Status         string `json:"status"`
	ExecutionError *struct {
		Message string `json:"message"`
	} `json:"executionError"`
	Transaction *struct {
		Digest string `json:"digest"`
	} `json:"transaction"`
	Checkpoint *struct {
		SequenceNumber json.RawMessage `json:"sequenceNumber"`
	} `json:"checkpoint"`
}

func statusFromEffects(digest string, effects *transactionEffects) (TransactionStatus, error) {
	status := TransactionStatus{Digest: digest, Outcome: TxPending}
	if effects.Transaction != nil && effects.Transaction.Digest != "" && effects.Transaction.Digest != digest {
		return TransactionStatus{}, fmt.Errorf(
			"Sui effects transaction digest %s does not match %s",
			effects.Transaction.Digest,
			digest,
		)
	}
	if effects.ExecutionError != nil {
		status.ExecutionError = effects.ExecutionError.Message
	}
	switch effects.Status {
	case "":
		status.Outcome = TxPending
	case "SUCCESS":
		if status.ExecutionError != "" {
			return TransactionStatus{}, errors.New("Sui SUCCESS effects unexpectedly include an execution error")
		}
		status.Outcome = TxSucceeded
	case "FAILURE":
		status.Outcome = TxFailed
	default:
		return TransactionStatus{}, fmt.Errorf("unknown Sui execution status %q", effects.Status)
	}
	if effects.Checkpoint != nil {
		sequence, err := parseGraphQLUint(effects.Checkpoint.SequenceNumber, "effects checkpoint sequence number")
		if err != nil {
			return TransactionStatus{}, err
		}
		status.CheckpointSequenceNumber = &sequence
	}
	return status, nil
}

func validateBase64(field, value string) error {
	value = strings.TrimSpace(value)
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return fmt.Errorf("decode Sui %s: %w", field, err)
	}
	if len(decoded) == 0 {
		return fmt.Errorf("Sui %s is required", field)
	}
	return nil
}

func parseGraphQLUint(raw json.RawMessage, field string) (uint64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return 0, fmt.Errorf("Sui GraphQL response is missing %s", field)
	}
	if strings.HasPrefix(value, `"`) {
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return 0, fmt.Errorf("decode Sui %s: %w", field, err)
		}
		value = decoded
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode Sui %s %q: %w", field, value, err)
	}
	return parsed, nil
}

type GraphQLError struct {
	Message    string                     `json:"message"`
	Path       []any                      `json:"path,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

type GraphQLErrors struct {
	Errors []GraphQLError
}

func (err *GraphQLErrors) Error() string {
	messages := make([]string, 0, len(err.Errors))
	for _, item := range err.Errors {
		messages = append(messages, item.Message)
	}
	return "Sui GraphQL errors: " + strings.Join(messages, "; ")
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (err *HTTPError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("Sui GraphQL HTTP status %d", err.StatusCode)
	}
	return fmt.Sprintf("Sui GraphQL HTTP status %d: %s", err.StatusCode, err.Body)
}

type graphQLRequest struct {
	OperationName string         `json:"operationName"`
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
}

func (adapter *GraphQLAdapter) post(
	ctx context.Context,
	operationName string,
	query string,
	variables map[string]any,
	output any,
) error {
	payload, err := json.Marshal(graphQLRequest{
		OperationName: operationName,
		Query:         query,
		Variables:     variables,
	})
	if err != nil {
		return fmt.Errorf("encode Sui GraphQL request: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		adapter.endpoint.String(),
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create Sui GraphQL request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := adapter.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxGraphQLResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Sui GraphQL response: %w", err)
	}
	if int64(len(body)) > maxGraphQLResponseBytes {
		return errors.New("Sui GraphQL response exceeds size limit")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &HTTPError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []GraphQLError  `json:"errors"`
	}
	if err = json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode Sui GraphQL response: %w", err)
	}
	if len(envelope.Errors) != 0 {
		return &GraphQLErrors{Errors: envelope.Errors}
	}
	if len(bytes.TrimSpace(envelope.Data)) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return errors.New("Sui GraphQL response is missing data")
	}
	if err = json.Unmarshal(envelope.Data, output); err != nil {
		return fmt.Errorf("decode Sui GraphQL data: %w", err)
	}
	return nil
}
