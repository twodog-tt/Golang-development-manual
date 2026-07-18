---
id: S-CONC-05
title: Channel 内部实现与有缓冲/无缓冲选型
module: runtime-concurrency
level: senior
frequency: 5
go_version: "1.22+"
tags: [channel, hchan, csp, communication]
status: published
code_refs:
  - basis/channel/main.go
sources:
  - https://go.dev/ref/spec#Channel_types
  - https://go.dev/src/runtime/chan.go
  - https://go.dev/blog/codelab-share
---

# Channel 内部实现与有缓冲/无缓冲选型

## 30 秒版（开场）

> Channel 底层是 **`hchan`**：环形队列 + `sendq/recvq` 等待链表 + 锁；**无缓冲**发送要与接收配对，**有缓冲**允许最多 `cap` 个值排队。无缓冲 send 返回只表示值已被接收方接走，不表示接收方已经完成业务处理。

## 3 分钟版（一面深度）

1. **是什么**：类型安全的 CSP 原语；`make(chan T)` 无缓冲，`make(chan T, n)` 有 n 槽环形缓冲。
2. **为什么**：用通信共享内存；runtime 统一调度阻塞与唤醒，与 netpoller/select 集成。
3. **怎么做**：阻塞式 send/recv 的慢路径会在 channel 锁下检查对端、缓冲和等待队列；可直接完成时不一定进入 park。无缓冲 channel 可与等待对端直接交接；有缓冲 channel 则按队列语义搬运。`close` 会唤醒等待接收者，也会使已阻塞或后续发送者 panic。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart TB
  subgraph hchan
    buf[ring buffer]
    sendq[send wait list]
    recvq[recv wait list]
  end
  S[send G] -->|缓冲满| sendq
  S -->|缓冲有空| buf
  R[recv G] -->|缓冲空| recvq
  R -->|有数据| buf
  sendq -.->|无缓冲时与接收操作配对| R
  recvq -.->|无缓冲时与发送操作配对| S
```

**关键字段（逻辑）**：`qcount/dataqsiz/buf/sendx/recvx`、等待 sudog 链表、`closed`。

**无缓冲**：send 与 recv **就绪配对**时，当前 runtime 可在两个 goroutine 的栈之间直接复制
元素并唤醒对端；这是实现细节。语言层保证相应的同步关系，不保证具体 `memmove` 路径。

**有缓冲**：send 在缓冲已满且没有可完成该操作的接收路径时阻塞；recv 在缓冲为空且没有
等待发送者时阻塞。缓冲满时若已有发送者排队，后续接收会先取缓冲头，再把等待发送者的值
补到缓冲尾，不能把所有情形都描述成 sendq 与 recvq 直接互拷。

**内存**：元素存于 `buf` 连续数组；`T` 含指针时 GC 扫描 channel。

**select**：规范保证多个可执行通信分支中做均匀伪随机选择。当前 runtime 还会建立独立的
poll/lock order，并按 channel 的排序键加锁以避免内部锁顺序死锁；排序和扫描方式属于实现
细节，不能当成业务公平性、优先级或长期无饥饿保证（见 S-CONC-07）。

## 生产场景

- **任务队列**：有缓冲 = 削峰；满则阻塞或 `select default` 丢弃/降级。
- **事件总线**：多订阅者需 **fan-out**（每消费者独立 chan 或 broadcast 模式），单 chan 多 reader 竞争。
- **优雅关闭**：`close(done)` 可广播停止信号；数据 channel 的关闭权应归属于明确的
  send-side 生命周期协调者。语言并没有“某个发送 goroutine 自动拥有 close 权”的规则，
  多生产者必须先协调完所有发送再关闭。

## 排查与工具

- goroutine profile：`chan receive` / `chan send` 栈顶
- trace：G 阻塞在 channel
- 指标：`len(ch)` 并发读取本身是安全的，但只是瞬时快照，不建立同步关系；可用于观测，不应用来做正确性判断

## 架构取舍

| 选型 | 适用 |
|------|------|
| 无缓冲 | 同步交接、限制生产者领先；若要确认“处理完成”还需单独 ACK |
| 小缓冲 | 平滑突发、固定背压 |
| 大缓冲 | 容忍短时消费慢，**掩盖慢消费者** |
| mutex+slice | 需 Peek、优先级、批量消费 |

**不宜用 channel**：跨进程、需持久化、复杂路由规则。

## 追问链

1. **关闭后 recv？** → 零值 + `ok=false`；send panic。
2. **nil channel？** → send/recv 永久阻塞（用于 select 禁用分支）。
3. **len/cap 含义？** → 当前元素数 / 缓冲容量。
4. **channel 并发安全吗？** → 多 goroutine send/recv 由 runtime 同步；但 close 与 send 必须由协议协调，否则可能 panic，不能靠 recover 当控制流。
5. **能广播吗？** → 单个值不会自动 fan-out；关闭 channel 可广播“一次性结束信号”，
   业务消息广播要为每个订阅者单独投递或使用 `sync.Cond`/消息总线。

## 反模式与事故

- 无缓冲 + 单 goroutine 自发自收 → **死锁**。
- 多生产者 close 同一 chan → panic。
- 超大 buffer 掩盖消费故障，OOM。

## 代码示例

```go
// 有缓冲背压
jobs := make(chan Job, 100)
go func() {
    for j := range jobs {
        process(j)
    }
}()
select {
case jobs <- j:
case <-ctx.Done():
    return ctx.Err()
}
```

可运行示例：[`basis/channel/main.go`](https://github.com/twodog-tt/Golang-development-manual/blob/master/basis/channel/main.go)（`firstChannel` / `secondChannel`）。

## 延伸阅读

- [Go spec: Channel types](https://go.dev/ref/spec#Channel_types)
- [runtime/chan.go](https://go.dev/src/runtime/chan.go)
- [A Tour of Go: Channels](https://go.dev/tour/concurrency/2)
- [Draveness：Go Channel 实现](https://draveness.me/golang/docs/part3-runtime/ch06-concurrency/golang-channel/)
