package cosmostx

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type cosmosFailureRoundTripper func(*http.Request) (*http.Response, error)

func (transport cosmosFailureRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestCosmosTransportFailuresDoNotBecomeTransactionStates(t *testing.T) {
	tests := []struct {
		name      string
		transport cosmosFailureRoundTripper
		timeout   time.Duration
	}{
		{
			name: "connection reset",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("read: connection reset by peer")
			},
			timeout: time.Second,
		},
		{
			name: "upstream timeout",
			transport: func(request *http.Request) (*http.Response, error) {
				<-request.Context().Done()
				return nil, request.Context().Err()
			},
			timeout: 15 * time.Millisecond,
		},
		{
			name: "latency exceeds deadline",
			transport: func(request *http.Request) (*http.Response, error) {
				select {
				case <-time.After(200 * time.Millisecond):
					return nil, errors.New("unexpected delayed response")
				case <-request.Context().Done():
					return nil, request.Context().Err()
				}
			},
			timeout: 15 * time.Millisecond,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := NewRPCAdapter("http://127.0.0.1:1", &http.Client{
				Transport: test.transport,
				Timeout:   test.timeout,
			})
			if err != nil {
				t.Fatal(err)
			}
			status, err := adapter.QueryTransaction(context.Background(), cosmosCompatibilityUnknownHash())
			if err == nil {
				t.Fatal("transport failure returned a transaction state")
			}
			if status.Outcome != "" {
				t.Fatalf("transport failure was classified as %q", status.Outcome)
			}
		})
	}
}

func TestCosmosWrongChainIdentityFailsClosed(t *testing.T) {
	readiness := NodeReadiness{Ready: true, ChainID: "untrusted-chain"}
	if err := readiness.ValidateChainID("trusted-chain"); err == nil {
		t.Fatal("wrong chain ID was accepted")
	}
}

func TestLocalnetCosmosTransportFaultDoesNotBecomeState(t *testing.T) {
	endpoint, _, client := cosmosLocalnetConfiguration(t)
	adapter, err := NewRPCAdapter(endpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	status, err := adapter.QueryTransaction(context.Background(), cosmosCompatibilityUnknownHash())
	if err == nil {
		t.Fatalf("injected transport fault returned state %+v", status)
	}
	if status.Outcome != "" {
		t.Fatalf("injected transport fault was classified as %q", status.Outcome)
	}
}
