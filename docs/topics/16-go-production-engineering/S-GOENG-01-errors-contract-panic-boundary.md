---
id: S-GOENG-01
title: 错误契约、Wrapping 与 Panic 边界
module: go-production-engineering
level: senior
frequency: 5
go_version: "1.22+"
tags: [errors, wrapping, panic, recover, api-design]
status: published
code_refs: []
sources:
  - https://go.dev/blog/go1.13-errors
  - https://go.dev/blog/errors-are-values
  - https://pkg.go.dev/errors
---

# 错误契约、Wrapping 与 Panic 边界

<a id="oral-card"></a>

## 要点卡

[返回高频核心锚点](../../high-frequency-roadmap.md)

!!! abstract "30 秒回答"

    Go 的 error 是 API 契约，不只是日志字符串。调用方需要分支处理的错误用稳定 sentinel、
    自定义类型或领域码表达，包装时用 `%w` 保留 cause，再通过 `errors.Is/As` 判断。`panic`
    只适合程序员错误或无法维持的不变量；`recover` 只能在同一 goroutine 的 defer 中生效，
    应放在 request/worker 隔离边界，记录栈并让当前工作单元失败，不能恢复后返回成功。

**3 分钟展开**

1. 先按调用方动作分类：业务拒绝、参数错误、可重试依赖故障、取消/超时和内部 bug。
2. repository/service 保留 cause 并增加 operation 上下文，transport adapter 再映射 HTTP/gRPC
   状态、MQ retry 或 DLQ。
3. 只有愿意把底层错误变成公开兼容契约时才 `%w` 暴露；否则转换成自己的领域错误。
4. 日志由真正处理或丢弃错误的边界记录一次，使用低基数分类码做指标，敏感数据不进入 error。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 错误分类决定调用方动作；包装不能破坏 `Is/As`；panic 后当前工作单元不得假装成功 |
| 手画图 | `repo cause → service classification → HTTP/gRPC/MQ mapping`；旁边画 `panic → boundary → fail closed` |
| 项目落点 | 链 RPC/HSM/发布 API：区分临时不可用、策略拒绝和内部不一致，分别决定重试、拒绝与告警 |
| 一个取舍 | 暴露底层 sentinel 方便调用方判断，却会把实现细节变成长期兼容承诺 |

**错误表达**

- ❌ “所有 error 都 `%w` 并每层打印；recover 后返回 nil，服务就不会崩。”
- ✅ “只暴露稳定契约并在处理边界记录一次；recover 要结束当前工作单元并 fail closed。”

**自测追问**：什么时候不用 `%w`？为什么一个 goroutine 不能 recover 另一个 goroutine 的 panic？

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  Repo["Repository error"] --> Wrap["%w + operation context"]
  Wrap --> Domain["Domain classification"]
  Domain --> Adapter{"Transport adapter"}
  Adapter --> HTTP["HTTP status + public code"]
  Adapter --> GRPC["gRPC code"]
  Adapter --> MQ["retry / DLQ"]
  Panic["panic"] --> Boundary["goroutine/request boundary recover"]
  Boundary --> Fail["fail closed + stack + metric"]
```

**稳定契约的三种方式**

| 方式 | 适合 | 注意 |
|------|------|------|
| sentinel `var ErrNotFound` | 少量、稳定、无额外字段 | 暴露后会成为兼容性承诺 |
| 自定义错误类型 | 需要字段，如 retry-after、资源 ID | 调用方用 `errors.As` |
| 领域分类码 | 跨进程协议、公开 API | 错误码与内部 cause 分离 |

```go
var ErrInsufficientBalance = errors.New("insufficient balance")

type RetryableError struct {
    After time.Duration
    Err   error
}

func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

func debit(ctx context.Context, repo Repository, id string) error {
    if err := repo.Debit(ctx, id); err != nil {
        return fmt.Errorf("debit account %s: %w", id, err)
    }
    return nil
}
```

`errors.Join` 表达“多个错误都发生了”，`errors.Is/As` 会遍历错误树；它不适合替代业务批处理结果，因为调用方通常还需要知道每个 item 的成功/失败。

**Typed nil 陷阱**

```go
func bad() error {
    var e *MyError
    return e // interface 的动态类型非 nil，因此 error != nil
}
```

应该直接 `return nil`，不要把 nil 指针装入 error interface。

## 生产场景

- DB 超时：保留 `context.DeadlineExceeded`，adapter 决定返回超时并统计依赖错误。
- 重复请求：返回稳定的幂等结果或 `ErrConflict`，不能只写 `"duplicate"`。
- Web3 签名服务：HSM 暂时不可用可重试；策略拒签是业务拒绝；签名不一致属于高危内部错误，应 fail closed。

## 排查与工具

- 日志记录 operation、request/trace ID、错误链；敏感参数和私钥材料绝不进入 error。
- 指标按低基数分类码聚合，错误原文只进日志。
- panic 记录栈、构建版本与请求 ID，并触发错误率告警；不要对所有 panic 自动重试。

## 架构取舍

公开库若 wrap 了某个底层 sentinel，调用方可能依赖它，之后更换底层实现就受兼容性约束。只有希望把底层错误纳入 API 契约时才 `%w`；否则可保留内部日志并返回自己的领域错误。

## 深挖问答

1. **什么时候不用 `%w`？** → 不希望暴露底层实现为公共契约时。
2. **`errors.Is` 和 `==`？** → `Is` 可沿 unwrap/join 树判断；`==` 只比较当前值。
3. **recover 后能继续吗？** → 被 recover 后，发生 panic 的函数不会从 panic 点继续；
   控制流按 defer/recover 规则返回到边界。边界应结束当前工作单元，未知不变量被破坏时
   不能继续提交副作用。
4. **context 错误要包装吗？** → 可以包装上下文，但必须让 `errors.Is(err, context.Canceled/DeadlineExceeded)` 仍成立。
5. **错误是否都应打印？** → 在“负责处理或丢弃”的边界打印一次，逐层重复打印会制造噪音。

## 反模式与事故

- `if strings.Contains(err.Error(), "duplicate")`：文本变化就破坏逻辑。
- 每层都 log + return：一条故障生成多份重复日志。
- `recover()` 后返回 nil：调用方误以为成功，可能造成资金状态错乱。
- 用 panic 处理用户输入或普通依赖超时。

## 延伸阅读

- [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
- [Errors are values](https://go.dev/blog/errors-are-values)
- [`errors` package](https://pkg.go.dev/errors)
