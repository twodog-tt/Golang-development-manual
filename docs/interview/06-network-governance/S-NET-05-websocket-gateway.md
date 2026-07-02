---
id: S-NET-05
title: WebSocket 网关设计
module: network-governance
level: senior
frequency: 3
go_version: "1.22+"
tags: [websocket, gateway, long-connection, push, gorilla]
status: published
code_refs: []
sources:
  - https://pkg.go.dev/github.com/gorilla/websocket
  - https://datatracker.ietf.org/doc/html/rfc6455
---

# WebSocket 网关设计

## 30 秒版（开场）

> **WebSocket** 在单 TCP 上全双工长连接，适合 **实时推送、IM、行情**。网关职责：**连接管理、鉴权、心跳、广播/房间、与 MQ 桥接**。Go 常用 **gorilla/websocket** 或 **nhooyr.io/websocket**。生产关键词：**单机连接上限、水平扩展、sticky session、背压、优雅下线**。

## 3 分钟版（一面深度）

1. **是什么**：HTTP Upgrade 握手后切换 WS 协议；服务端可主动 push；帧类型 Text/Binary/Ping/Pong/Close。
2. **为什么**：轮询浪费；SSE 仅服务端推且 HTTP/1.1 连接数受限；gRPC stream 浏览器不原生支持。
3. **怎么做**：独立 **WS Gateway** 集群；握手阶段 JWT 鉴权；`Ping` 心跳 + 读超时踢死连接；**Hub** 管理 `map[userID]*Conn`；业务事件经 **Redis Pub/Sub 或 Kafka**  fan-out 到各节点；K8s **Ingress sticky** 或客户端重连。

## 10 分钟版（原理 + 图示）

**架构**

```mermaid
flowchart TB
  Client[Browser/App] -->|WSS| GW1[WS Gateway Pod]
  Client --> GW2[WS Gateway Pod]
  GW1 --> Hub[Hub: userID to Conn]
  GW2 --> Hub2[Hub]
  Kafka[Kafka topic] --> GW1
  Kafka --> GW2
  API[Business API] --> Kafka
  Redis[Redis Pub/Sub] -.->|跨节点广播| GW1
  Redis -.-> GW2
```

**连接管理**：每连接 **读 goroutine + 写 goroutine**（或 send channel）；写集中避免 concurrent Write；`SetReadLimit` 防大包；`SetPongHandler` 续读 deadline。

**扩展**：连接状态 **本地内存**；跨 Pod 推送用 Pub/Sub 按 `user_id` channel；百万连接需 **分片网关 + 专用 LB（如 L4）**；C10K Go 可行但注意 **每连接 2 goroutine** 可改 epoll 风格库。

**优雅下线**：SIGTERM → 停 accept → 广播 close frame → 等待 drain 或超时强关 → 客户端指数退避重连。

## 生产场景

- **订单状态推送**：支付成功 → MQ → 网关查 user 是否在线 → WS push JSON。
- **在线客服 IM**：房间 `room_id` Hub；消息持久化仍走 HTTP/DB，WS 仅实时层。
- **大屏行情**：广播多；可 **合并 tick** 降频，二进制 Protobuf 减带宽。

## 排查与工具

| 工具 | 用途 |
|------|------|
| `netstat` / conntrack | 连接数 |
| pprof goroutine | 泄漏连接 |
| Prometheus | 在线数、推送延迟 |
| wscat | 手工测握手 |

路径：大量断连 → LB idle 超时 vs 心跳间隔 → 是否 concurrent Write → OOM 看连接是否泄漏未 Unregister。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| 独立 WS 网关 | 连接与 HTTP API 解耦 | 极小流量 |
| Redis Pub/Sub | 跨节点 push | 强可靠（用 Kafka） |
| SSE | 单向通知、简单 | 双向 IM |
| 第三方（Pusher/Ably） | 快速上线 | 成本/定制 |
| gRPC stream 内网 | 非浏览器 | 公网 |

## 追问链

1. **握手过程？** → Upgrade 101，Sec-WebSocket-Accept 校验。
2. **如何鉴权？** → Query token 或首帧 auth 消息；握手前验 JWT。
3. **心跳怎么做？** → Ping/Pong 或应用层 JSON ping；读 deadline 续期。
4. **多机如何推指定用户？** → 本地有则写；无则 Pub/Sub 到持有连接节点。
5. **和 HTTP/2 server push 区别？** → WS 全双工独立协议；H2 push 已弱化。

## 反模式与事故

- 多 goroutine 同时 `WriteMessage`——panic 或帧错乱。
- 无读超时——半开连接占满 FD。
- 广播单线程写万连接——阻塞；应 per-conn buffer + drop 慢客户端。
- 网关当 DB——消息未落库即 push，断线丢失。

## 代码示例

```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true }, // 生产应校验 Origin
    ReadBufferSize: 1024, WriteBufferSize: 1024,
}

type Client struct {
    conn *websocket.Conn
    send chan []byte
}

func (c *Client) writePump() {
    ticker := time.NewTicker(30 * time.Second)
    defer c.conn.Close()
    for {
        select {
        case msg, ok := <-c.send:
            if !ok {
                _ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            _ = c.conn.WriteMessage(websocket.TextMessage, msg)
        case <-ticker.C:
            _ = c.conn.WriteMessage(websocket.PingMessage, nil)
        }
    }
}
```

Hub 维护 `register/unregister/broadcast` channel，见常见 gorilla chat demo 模式。

## 延伸阅读

- [RFC 6455 WebSocket](https://datatracker.ietf.org/doc/html/rfc6455)
- [gorilla/websocket](https://github.com/gorilla/websocket)
- [gorilla/websocket 文档](https://pkg.go.dev/github.com/gorilla/websocket)
