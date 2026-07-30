---
id: S-CODE-04
title: errgroup 语义实现
module: coding-senior
level: senior
frequency: 4
go_version: "1.22+"
tags: [errgroup, context, concurrency, handwriting]
status: published
code_refs:
  - examples/senior/errgroup/errgroup.go
  - examples/senior/errgroup/errgroup_test.go
sources:
  - https://pkg.go.dev/golang.org/x/sync/errgroup
  - https://go.dev/blog/context
---

# errgroup 语义实现

<a id="oral-card"></a>

## 要点卡

[返回 P0 知识图谱](../_meta/p0-knowledge-graph.md)

!!! abstract "30 秒回答"

    **errgroup** = **WaitGroup + 首个 error + Context 取消**。`WithContext` 派生可取消 ctx；任一 `Go(fn)` 返回 error → **`errOnce` 记录 + cancel**；`Wait` 等全部结束并返回该 error。讲解关键词：**errOnce、cancel 传播、Wait 后再 cancel 一次无害**。

**3 分钟展开**

1. **是什么**：`golang.org/x/sync/errgroup` 简化「多 goroutine 有一失败则全员停工」。
2. **为什么**：并行调多个 RPC/查多个库，一个失败不应继续浪费资源（见 [S-CONC-17 Pipeline](../01-runtime-concurrency/S-CONC-17-pipeline.md)）。
3. **怎么做**：`WaitGroup` 计数；`Go` 里 `defer Done()`；error 时 `errOnce.Do` 存 err 并 `cancel()`；子任务 `select ctx.Done()`。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 首个非 nil error 触发派生 ctx 取消；Wait 仍等待所有已启动任务；任务必须主动观察取消 |
| 手画图 | `Go A/B/C → first error → cancel ctx → cooperative exits → Wait → first error` |
| 项目落点 | 用实际并行 RPC、链节点查询或批量校验说明 fail-fast 与下游请求取消；只引用本人实际参与部分和可解释指标 |
| 一个取舍 | fail-fast 节省资源但可能丢失完整错误集合；需要逐项结果时应换聚合语义 |

**错误表达**

- ❌ “errgroup 一出错就强制杀死其他 goroutine，并立刻从 Wait 返回；还能自动回滚副作用。”
- ✅ “取消是协作信号，Wait 要等任务返回；事务补偿、panic 和部分结果都需业务层另行设计。”

**自测追问**：函数忽略 ctx 永久阻塞时 errgroup 能做什么？SetLimit 限制的是 active 还是所有排队调用？

## 10 分钟版（原理 + 图示）

```mermaid
flowchart TD
  Start[WithContext] --> G1[Go task A]
  Start --> G2[Go task B]
  G2 -->|error| Cancel[cancel ctx]
  Cancel --> G1
  G1 -->|ctx.Done| Stop[提前退出]
  G1 --> Wait[Wait]
  G2 --> Wait
  Wait --> Ret[return first err]
```

**与 WaitGroup 对比**

| | WaitGroup | errgroup |
|---|-----------|----------|
| 错误 | 需自行 channel/atomic | 内置首个 error |
| 取消 | 需自建 ctx | 内置 cancel |
| 适用 | 纯等待 | 并行任务有失败语义 |

**手写要点**

```go
type Group struct {
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	errOnce sync.Once
	err     error
}
```

- `errOnce` 保证只记录 **第一个** error
- `Wait()` 末尾再 `cancel()`：清理未触发 error 的路径（与官方实现类似）

## 生产场景

- 启动时并行 ping 多个依赖
- 批量导出：任一分片失败则中止
- 与 `gin-example/example_28` errgroup 多服务启动同类

## 排查与工具

- `go test ./errgroup/...`
- 生产直接用 `golang.org/x/sync/errgroup`

## 架构取舍

| 方案 | 适用 |
|------|------|
| 手写 errgroup | 编码练习 / 中等负载 |
| x/sync/errgroup | 生产 |
| channel 收 err | 需收集 **所有** 错误时 |

## 深挖问答

1. **为何 errOnce？** → 多 goroutine 同时失败，只保留首个有意义。
2. **子任务如何感知取消？** → `fn` 内 `select <-ctx.Done()` 或传 ctx 给 HTTP/DB。
3. **SetLimit(n) 呢？** → 官方扩展：`Go` 在达到上限时会阻塞；有任务运行时不能修改 limit。若不希望阻塞提交方可看 `TryGo`。
4. **Wait 返回 nil 但 ctx 已 cancel？** → `Wait` 只返回任务函数报告的首个 error。若 parent 已取消但所有函数都忽略取消并返回 nil，`Wait` 仍可返回 nil；任务应把 `ctx.Err()` 向上返回。

## 反模式与事故

- 官方 `errgroup` 不把 panic 自动转成 error。只应在明确的进程/请求边界按策略 recover、保留堆栈并告警；不要在每个任务里无差别吞掉 panic
- **不用 ctx 仍调阻塞 IO** → cancel 无效，Wait  hung
- **在 Go 外再开 goroutine 不 Wait** → 泄漏

## 代码示例

见 [examples/senior/errgroup/errgroup.go](https://github.com/twodog-tt/Golang-development-manual/blob/master/examples/senior/errgroup/errgroup.go)：

```go
func (g *Group) Go(fn func() error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				g.cancel()
			})
		}
	}()
}
```

```bash
cd examples/senior && go test ./errgroup/...
```

## 延伸阅读

- [x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [Go Context 博客](https://go.dev/blog/context)
