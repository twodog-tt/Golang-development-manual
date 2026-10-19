// Package server demonstrates trusted identities and resource-scoped MCP tools.
package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Principal struct {
	Subject, Tenant string
	Scopes          []string
}
type Config struct {
	Authority *Authority
	// LocalToken is supplied by the trusted launcher, never tool arguments.
	LocalToken string
	Timeout    time.Duration
	Query      func(context.Context, string, string) (Order, error)
}
type Order struct {
	OrderID     string `json:"order_id"`
	Status      string `json:"status"`
	AmountMinor int64  `json:"amount_minor"`
}

func New(c Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "golang-manual-mcp", Version: "1.1.0"}, nil)
	if c.Timeout <= 0 {
		c.Timeout = 200 * time.Millisecond
	}
	if c.Query == nil {
		c.Query = query
	}
	principal := func(req *mcp.CallToolRequest, scope string) (Principal, error) {
		if c.Authority == nil {
			return Principal{}, errors.New("permission denied")
		}
		token := c.LocalToken
		if req.Extra != nil { // populated by the HTTP transport, not model arguments
			if req.Extra.TokenInfo == nil {
				return Principal{}, errors.New("permission denied")
			}
			fields := strings.Fields(req.Extra.Header.Get("Authorization"))
			if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
				return Principal{}, errors.New("permission denied")
			}
			token = fields[1]
		}
		p, err := c.Authority.Resolve(token)
		if req.Extra != nil && req.Extra.TokenInfo != nil && req.Extra.TokenInfo.UserID != fmt.Sprintf("%d:%s%s", len(p.Tenant), p.Tenant, p.Subject) {
			return Principal{}, errors.New("permission denied")
		}
		if err != nil || !slices.Contains(p.Scopes, scope) {
			return Principal{}, errors.New("permission denied")
		}
		return p, nil
	}
	mcp.AddTool(s, &mcp.Tool{Name: "greet", Description: "向指定名字打招呼"}, func(ctx context.Context, req *mcp.CallToolRequest, in greetInput) (*mcp.CallToolResult, greetOutput, error) {
		if _, err := principal(req, "greet"); err != nil {
			return nil, greetOutput{}, err
		}
		return greet(ctx, req, in)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_order", Description: "查询当前授权租户的演示订单（Mock 数据）"}, func(ctx context.Context, req *mcp.CallToolRequest, in getOrderInput) (*mcp.CallToolResult, Order, error) {
		p, err := principal(req, "orders:read")
		if err != nil {
			return nil, Order{}, err
		}
		ctx, cancel := context.WithTimeout(ctx, c.Timeout)
		defer cancel()
		out, err := c.Query(ctx, p.Tenant, in.OrderID)
		if ctx.Err() != nil {
			return nil, Order{}, ctx.Err()
		}
		return nil, out, err
	})
	return s
}
func Run(ctx context.Context, srv *mcp.Server) error { return srv.Run(ctx, &mcp.StdioTransport{}) }

type greetInput struct {
	Name string `json:"name" jsonschema:"要问候的人名"`
}
type greetOutput struct {
	Greeting string `json:"greeting"`
}

func greet(_ context.Context, _ *mcp.CallToolRequest, in greetInput) (*mcp.CallToolResult, greetOutput, error) {
	name := in.Name
	if name == "" {
		name = "friend"
	}
	return nil, greetOutput{Greeting: fmt.Sprintf("Hello, %s (from Go MCP)", name)}, nil
}

type getOrderInput struct {
	OrderID string `json:"order_id" jsonschema:"订单号"`
}

func query(ctx context.Context, tenant, id string) (Order, error) {
	if err := ctx.Err(); err != nil {
		return Order{}, err
	}
	orders := map[string]map[string]Order{"tenant-a": {"10001": {OrderID: "10001", Status: "paid", AmountMinor: 9900}}, "tenant-b": {"10002": {OrderID: "10002", Status: "shipped", AmountMinor: 19900}}}
	if o, ok := orders[tenant][id]; ok {
		return o, nil
	}
	return Order{}, errors.New("order unavailable") // no cross-tenant existence leak
}
