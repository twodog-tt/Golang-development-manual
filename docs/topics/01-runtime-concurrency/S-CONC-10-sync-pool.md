---
id: S-CONC-10
title: sync.Pool 与 GC 交互
module: runtime-concurrency
level: senior
frequency: 4
go_version: "1.22+"
tags: [sync.Pool, gc, object-reuse, allocation]
status: published
code_refs: []
sources:
  - https://pkg.go.dev/sync#Pool
  - https://go.dev/src/runtime/mgc.go
  - https://go.dev/blog/go1.13
---

# sync.Pool 与 GC 交互

## 30 秒版（开场）

> **sync.Pool** 是按 P 优化的临时对象缓存，`Get` 随时可能拿不到旧对象。GC 开始时当前 local 会转为 victim、上一轮 victim 被丢弃，因此对象可能跨一轮 GC 被复用，但绝不能把 Pool 当持久缓存或资源池。

## 3 分钟版（精讲深度）

1. **是什么**：`Get/Put` 复用 `any`；内部 `poolLocal` 数组按 P 分片。
2. **为什么**：高频短生命周期对象（buffer、临时 slice）降低 GC 负担。
3. **怎么做**：`New` 提供缺省构造；Put 前 **重置对象状态**；对象可能被 GC 随时回收。

## 10 分钟版（原理 + 图示）

```mermaid
sequenceDiagram
  participant G as Goroutine
  participant Pool as sync.Pool
  participant GC as GC cycle
  G->>Pool: Get()
  Pool-->>G: reused or New()
  G->>Pool: Put(reset obj)
  GC->>Pool: current local → victim，旧 victim 丢弃
  Note over Pool: 下轮 Get 可能全走 New
```

**GC 关系（1.13+ victim 机制简化理解）**

- victim 机制让部分对象有机会再存活一轮 GC，但 runtime 可以在任意时刻移除 Pool 中的对象。
- 正确语义只依赖 `Get` 可能返回 nil/调用 `New`，不要依赖“对象至少保留几轮”。

**适用对象**：`bytes.Buffer`、`[]byte`、解码临时结构。

**禁止**：数据库连接、带 goroutine 的对象、未 Reset 的请求上下文。

## 生产场景

- **JSON/Proto 序列化**：`buf := pool.Get().(*bytes.Buffer); buf.Reset()`
- **事故**：Pool 存 `*Request` 未清字段，下一请求读到上一用户 PII。
- **指标**：关注 allocs/op、B/op、GC CPU 与 P99；Pool 可能降低分配和 GC 工作，但不保证 STW 一定缩短。

## 排查与工具

- `go test -bench` + `alloc_space`
- `GODEBUG=gctrace=1` 对比开 Pool 前后
- pprof alloc_objects

## 架构取舍

| 方案 | 适用 |
|------|------|
| sync.Pool | 临时对象、可 Reset、丢失可接受 |
| 固定 buffer 栈上/线程本地 | 小对象、生命周期清晰 |
| 连接池（sql.DB） | 长生命周期资源 |
| arena（实验） | 批量分配一次性释放 |

## 深挖问答

1. **Pool 线程安全吗？** → 是，但 Put/Get 的对象本身需无竞态。
2. **GC 如何处理 Pool？** → current local 转 victim，旧 victim 清除；所以缓存保留非确定。
3. **New 何时调用？** → Get 时本地与 victim 皆空。
4. **能统计池大小吗？** → 无公开 API。
5. **和 free list 区别？** → Pool 与 GC 协作、非确定性保留。

## 反模式与事故

- 用 Pool **缓存业务实体** 当 LRU。
- Put `[]byte` 后外部仍在读写 → 同一底层数组可能被其他请求复用，产生数据竞态或跨请求数据污染。
- 压测只测稳态，忽略 **GC 后延迟尖刺**。

## 代码示例

```go
var bufPool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}

func Encode(v any) ([]byte, error) {
    b := bufPool.Get().(*bytes.Buffer)
    b.Reset()
    defer bufPool.Put(b)
    if err := json.NewEncoder(b).Encode(v); err != nil {
        return nil, err
    }
    return append([]byte(nil), b.Bytes()...), nil
}
```

## 延伸阅读

- [sync.Pool 文档](https://pkg.go.dev/sync#Pool)
- [Go 1.13 release notes - Pool](https://go.dev/doc/go1.13)
- [Draveness：sync.Pool 实现](https://draveness.me/golang/docs/part3-runtime/ch07-memory/golang-sync-pool/)
