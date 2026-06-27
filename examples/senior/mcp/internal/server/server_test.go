package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCP_GetOrderTool(t *testing.T) {
	srv := New()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)

	cTransport, sTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	go func() {
		if err := srv.Run(ctx, sTransport); err != nil {
			t.Errorf("server: %v", err)
		}
	}()

	session, err := client.Connect(ctx, cTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_order",
		Arguments: map[string]any{"order_id": "10001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatal("tool returned error")
	}

	var out struct {
		OrderID string  `json:"order_id"`
		Status  string  `json:"status"`
		Amount  float64 `json:"amount"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "paid" || out.Amount != 99 {
		t.Fatalf("got %+v", out)
	}
}

func TestMCP_GreetTool(t *testing.T) {
	srv := New()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)

	cTransport, sTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	go func() {
		srv.Run(ctx, sTransport)
	}()

	session, err := client.Connect(ctx, cTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"name": "Go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text == "" {
		t.Fatal("empty greeting")
	}
}
