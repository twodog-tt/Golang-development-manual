// Package main 演示 MCP Server（stdio），供 Cursor / Claude Desktop 挂载。
//
// 运行：go run ./examples/senior/mcp/
// Cursor MCP 配置示例见 docs/interview/10-ai-engineering/S-AI-07-mcp-server-go.md
package main

import (
	"context"
	"log"

	"td-homework/examples/senior/mcp/internal/server"
)

func main() {
	if err := server.Run(context.Background(), server.New()); err != nil {
		log.Fatal(err)
	}
}
