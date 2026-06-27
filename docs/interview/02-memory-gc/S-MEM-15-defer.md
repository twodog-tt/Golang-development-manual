---
id: S-MEM-15
title: defer 链、开销与错误处理
module: memory-gc
level: senior
frequency: 4
go_version: "1.22+"
tags: [defer, panic, error-handling, performance]
status: published
code_refs: []
sources:
  - https://go.dev/ref/spec#Defer_statements
  - https://go.dev/doc/effective_go#defer
  - https://go.dev/blog/defer-panic-and-recover
---

# defer 链、开销与错误处理

## 30 秒版（开场）

> **defer** 注册函数在 **return 之前、panic 传播之前** LIFO 执行；实现为 **defer 链表**（栈上 open defer 或堆上 defer 结构，视版本与逃逸而定）。热路径大量 defer 有开销；错误处理常用 **defer + named return +闭包捕获 err**。生产关键词：**open defer、1.14 优化、别在循环里无脑 defer**。

## 3 分钟版（一面深度）

1. **是什么**：推迟调用到 surrounding function 退出；参数在 defer 语句处求值（除函数字面量闭包延迟读变量）。
2. **为什么**：保证资源释放（close、Unlock、tx.Rollback）路径统一，避免漏释。
3. **怎么做**：锁/文件/连接用 defer；循环内改用显式 close 或封装函数；错误用 `defer func(){ if err!=nil { rollback() } }()`。

## 10 分钟版（原理 + 图示）

**执行顺序**

```go
defer fmt.Println(1)
defer fmt.Println(2)
// return 时打印 2 然后 1
```

**与 return 交互**

- 先算 return 右值 → 赋给 named return → 跑 defer 链 → 再真正 ret。
- defer 可修改 **named result** 影响最终返回值。

```mermaid
flowchart TD
  Ret[return 语句] --> Defers[defer LIFO]
  Defers --> Panic{panic?}
  Panic -->|是| Recover[若有 recover]
  Panic -->|否| Exit[函数退出]
```

**开销（面试表述）**

- 早期：堆分配 defer 结构；1.13+ **open defer** 优化部分场景减分配。
- 极热微函数：百万次 defer 可测到 ns 级差异，通常不是首要瓶颈。
- **循环中 defer**：defer 累积到函数结束才执行，可能耗尽 fd/连接。

**panic/recover**

- `recover` 仅在 **defer 直接调用的函数栈** 内有效。
- 业务错误用 `error` 返回，非 panic。

## 生产场景

- **DB 事务**：`defer tx.Rollback()` + 成功 `Commit` 覆盖；注意 Rollback 忽略 ErrTxDone。
- **HTTP handler**：循环里 `defer resp.Body.Close()` 导致连接池耗尽。
- **可观测**：fd 泄漏、too many open files；pprof 见 defer 相关分配（通常次要）。

## 排查与工具

| 工具 | 用途 |
|------|------|
| 代码审查 | 循环 defer |
| pprof allocs | 极端 defer 热点 |
| vet/staticcheck | 常见资源泄漏 |

路径：连接/fd 涨 → 搜 defer Close → 改子函数或显式 close → 压测连接数稳定。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| defer Close/Unlock | 绝大多数 IO/锁 | 纳秒级循环内核 |
| 子函数 + defer | 循环体资源 | 过度嵌套 |
| 显式 close | 热循环 | 多 return 易漏 |
| errgroup + context | 并发生命周期 | 单函数资源 |

## 追问链

1. **defer 参数何时求值？** → 注册时，除闭包读外部变量在执行时。
2. **defer 修改返回值？** → 仅 named return 可被 defer 内赋值影响。
3. **recover 能跨 goroutine 吗？** → 不能，只在同一 G 的 defer 栈。
4. **defer 与 os.Exit？** → Exit 跳过 defer。
5. **1.14+ defer 性能？** → open defer 减开销，实现细节口述「持续优化」即可。

## 反模式与事故

- `for { f, _ := os.Open(); defer f.Close() }` —— 经典 fd 泄漏题。
- 用 panic/recover 做正常流程控制。
- `defer mu.Unlock()` 前 early return 忘记 err 路径仍执行 Unlock——其实 defer 正确，反而说明 defer 价值。

## 代码示例

```go
func writeFile(path string, data []byte) (err error) {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer func() {
        cerr := f.Close()
        if err == nil {
            err = cerr
        }
    }()
    _, err = f.Write(data)
    return err
}

// 循环：用子函数确保 defer 在每次迭代结束执行
func processFiles(paths []string) error {
    for _, p := range paths {
        if err := func() error {
            f, err := os.Open(p)
            if err != nil {
                return err
            }
            defer f.Close()
            return consume(f)
        }(); err != nil {
            return err
        }
    }
    return nil
}
```

## 延伸阅读

- [Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover)
- [Go spec: Defer statements](https://go.dev/ref/spec#Defer_statements)
- [Open defer 实现（Go 1.14）](https://go.dev/doc/go1.14)
