package suiadapter

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type suiFailureRoundTripper func(*http.Request) (*http.Response, error)

func (transport suiFailureRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestSuiTransportFailuresDoNotBecomeTransactionStates(t *testing.T) {
	tests := []struct {
		name      string
		transport suiFailureRoundTripper
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
			adapter, err := NewGraphQLAdapter("http://127.0.0.1:1/graphql", &http.Client{
				Transport: test.transport,
				Timeout:   test.timeout,
			})
			if err != nil {
				t.Fatal(err)
			}
			status, err := adapter.QueryTransaction(context.Background(), suiCompatibilityUnknownDigest())
			if err == nil {
				t.Fatal("transport failure returned a transaction state")
			}
			if status.Outcome != "" {
				t.Fatalf("transport failure was classified as %q", status.Outcome)
			}
		})
	}
}

func TestSuiWrongChainIdentityFailsClosed(t *testing.T) {
	readiness := NodeReadiness{Ready: true, ChainIdentifier: "untrusted-chain"}
	if err := readiness.ValidateChainIdentifier("trusted-chain"); err == nil {
		t.Fatal("wrong chain identifier was accepted")
	}
}

func TestLocalnetSuiTransportFaultDoesNotBecomeState(t *testing.T) {
	endpoint, _, client := suiLocalnetConfiguration(t)
	adapter, err := NewGraphQLAdapter(endpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	if support := adapter.EndpointSupport(); support.DeprecatedJSONRPC || support.Transport != "GraphQL" {
		t.Fatalf("fault test crossed deprecated transport boundary: %+v", support)
	}
	status, err := adapter.QueryTransaction(context.Background(), suiCompatibilityUnknownDigest())
	if err == nil {
		t.Fatalf("injected transport fault returned state %+v", status)
	}
	if status.Outcome != "" {
		t.Fatalf("injected transport fault was classified as %q", status.Outcome)
	}
}
