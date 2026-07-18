---
id: S-NET-01
title: gRPC vs HTTP/REST 选型
module: network-governance
level: senior
frequency: 4
go_version: "1.22+"
tags: [grpc, rest, http2, protobuf, api-design]
status: published
code_refs: []
sources:
  - https://grpc.io/docs/languages/go/
  - https://pkg.go.dev/google.golang.org/grpc#NewClient
  - https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
  - https://protobuf.dev/programming-guides/proto3/#updating
  - https://connectrpc.com/docs/go/getting-started
  - https://google.github.io/styleguide/jsoncstyleguide.html
---

# gRPC vs HTTP/REST 选型

## 30 秒版（开场）

> **gRPC** 通常使用 HTTP/2 + Protobuf，强类型、支持双向流与多路复用，适合 **内部微服务**。**REST/JSON** 人类可读、浏览器友好、生态广，适合 **对外 API/BFF**。Go 可用 `google.golang.org/grpc`；需要同时服务浏览器和多种 RPC 协议时可考虑 **Connect**。生产关键词：**超时、重试幂等、负载均衡、TLS/mTLS**。

## 3 分钟版（一面深度）

1. **是什么**：gRPC 是 RPC 框架，IDL 定义 service/method，二进制 Protobuf 序列化；REST 以资源为中心，HTTP 动词 + JSON，常 OpenAPI 描述。
2. **为什么**：内网高 QPS、低延迟、流式推送选 gRPC；公网开放平台、调试成本、CDN 缓存选 REST。
3. **怎么做**：服务间 gRPC；按网络环境谨慎配置 **keepalive**；通过 DNS/headless service、自定义 resolver/xDS 或 L7 proxy 做负载均衡；边缘 REST；需要浏览器直调时用 gRPC-Web 或 Connect；统一 **context deadline、metadata/trace context 透传**。

## 10 分钟版（原理 + 图示）

**对比**

| 维度 | gRPC | REST/JSON |
|------|------|-----------|
| 协议 | grpc-go 通常为 HTTP/2 | REST 是架构风格，可承载于 HTTP/1.1、2 或 3 |
| 载荷 | Protobuf 小 | JSON 大 |
| 契约 | .proto 强类型 | OpenAPI 可选 |
| 流 | 双向流 | SSE/WebSocket 补充 |
| 调试 | grpcurl/反射 | curl/Postman |
| 缓存 | 不友好 | HTTP 缓存语义 |

```mermaid
flowchart LR
  Mobile[Mobile/Web] -->|HTTPS JSON| BFF[BFF / API Gateway]
  BFF -->|gRPC| Order[Order Svc]
  BFF -->|gRPC| User[User Svc]
  Order -->|gRPC stream| Event[Event Svc]
```

**gRPC Go 要点**：每次调用传入有 deadline 的 `context`；keepalive 参数应与服务端策略协调，过于激进会收到 `GOAWAY: too_many_pings`；interceptor/stats handler 做 auth、metrics、trace；重试只对 **幂等或有幂等键** 的方法；服务端用 `status.Error(codes.NotFound, ...)` 表达 gRPC 状态，只有经过 gateway/Connect 等协议转换层时才会映射为 HTTP 状态。

**REST 要点**：版本 `/v1/`；Problem Details 错误体；`ETag` 缓存；HSTS；限流在网关。

## 生产场景

- **订单调支付/库存**：内网 gRPC，P99 < 10ms，Protobuf 省带宽。
- **开放平台给第三方**：REST + OAuth2 + Webhook（JSON）。
- **实时推送**：gRPC server stream 或独立 WebSocket 网关。

## 排查与工具

| 工具 | 用途 |
|------|------|
| grpcurl | 无客户端调 RPC |
| grpc-go stats/prometheus | 延迟、错误码 |
| OpenTelemetry | 跨协议 trace |
| wireshark/http2 | 帧级调试 |

路径：超时增多 → 查 client deadline vs server 处理时间 → keepalive 是否触发 GOAWAY → LB 是否长连接亲和。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| gRPC 内网 | 微服务主力 | 浏览器直连 |
| REST 公网 | 开放 API | 超高频二进制 |
| Connect | 一套 handler 多协议 | 纯 legacy |
| GraphQL BFF | 聚合多源 | 简单 CRUD |
| 消息异步 | 解耦峰值 | 同步查询 |

## 追问链

1. **HTTP/2 好处？** → 多路复用、头部压缩、单连接并发 stream。
2. **gRPC 负载均衡？** → 客户端 resolver + LB policy，或 L7 proxy/mesh。直接连 ClusterIP 时，长寿命 HTTP/2 连接可能长期落在少数后端。
3. **Protobuf 如何兼容演进？** → 不改、不复用既有字段号；删除字段后 `reserved` 字段号和名称；新增字段通常是二进制 wire-safe，但类型变更只能按官方 wire compatibility 规则评估，不能笼统说“都可改”。
4. **REST 如何实现幂等？** → Idempotency-Key header + 服务端去重。
5. **mTLS？** → 双向证书，零信任内网。

## 反模式与事故

- 公网暴露 gRPC 无 TLS——明文与反射信息泄露。
- 重试非幂等 CreateOrder——重复下单。
- proto 改字段编号——线上解码错乱。
- REST 返回 200 包 error 字段——监控无法按状态码告警。

## 代码示例

```go
// gRPC 客户端：TLS + OTel + 单次调用 deadline
conn, err := grpc.NewClient(
    "dns:///order-svc:50051",
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
if err != nil {
    return err
}
defer conn.Close()

client := pb.NewOrderClient(conn)
callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
defer cancel()
resp, err := client.GetOrder(callCtx, &pb.GetOrderRequest{Id: id})
```

## 延伸阅读

- [gRPC Go Quick Start](https://grpc.io/docs/languages/go/quickstart/)
- [Connect Go](https://connectrpc.com/docs/go/getting-started)
- [Google API Design Guide](https://cloud.google.com/apis/design)
