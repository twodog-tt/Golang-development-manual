// Package server 封装可测试的 MCP Server 构建逻辑（S-AI-07 示例）。
package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// New 创建带演示工具的 MCP Server。
func New() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "golang-manual-mcp",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "greet",
		Description: "向指定名字打招呼",
	}, greet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_order",
		Description: "查询演示订单状态（Mock 数据）",
	}, getOrder)

	return s
}

// Run 在 stdio 传输上启动服务。
func Run(ctx context.Context, srv *mcp.Server) error {
	return srv.Run(ctx, &mcp.StdioTransport{})
}

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

type getOrderOutput struct {
	OrderID string  `json:"order_id"`
	Status  string  `json:"status"`
	Amount  float64 `json:"amount"`
}

func getOrder(_ context.Context, _ *mcp.CallToolRequest, in getOrderInput) (*mcp.CallToolResult, getOrderOutput, error) {
	// Mock 订单表，演示 Tool 返回结构化 JSON
	mock := map[string]getOrderOutput{
		"10001": {OrderID: "10001", Status: "paid", Amount: 99.0},
		"10002": {OrderID: "10002", Status: "shipped", Amount: 199.0},
	}
	if o, ok := mock[in.OrderID]; ok {
		return nil, o, nil
	}
	return nil, getOrderOutput{OrderID: in.OrderID, Status: "not_found", Amount: 0}, nil
}
