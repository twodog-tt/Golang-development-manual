---
id: S-MEM-10
title: pprof heap/allocs 实战解读
module: memory-gc
level: senior
frequency: 5
go_version: "1.22+"
tags: [pprof, heap, allocs, profiling]
status: published
code_refs: []
sources:
  - https://pkg.go.dev/net/http/pprof
  - https://go.dev/doc/diagnostics
  - https://github.com/google/pprof
---

# pprof heap/allocs 实战解读

## 30 秒版（开场）

> **heap** 看当前存活（inuse），**allocs** 看历史累计分配（含已释放）；排查泄漏用 heap，排查 GC 压力用 allocs。采样基于 **rate** 默认 512KB 一次。生产关键词：**inuse_space vs alloc_space、top -cum、base profile 对比**。

## 3 分钟版（一面深度）

1. **是什么**：`/debug/pprof/heap` 暴露内存 profile；`go tool pprof` 交互分析调用栈。
2. **为什么**：定位谁占内存、谁分配多；比 `ReadMemStats` 多了栈归因。
3. **怎么做**：`?gc=1` 强制 GC 后快照；`-sample_index=inuse_space`；对比两次 heap 差分；allocs 看 `alloc_objects`。

## 10 分钟版（原理 + 图示）

**profile 类型**

| 模式 | 含义 | 典型问题 |
|------|------|----------|
| inuse_space | 当前占用字节 | 泄漏、缓存过大 |
| inuse_objects | 当前对象数 | 小对象过多 |
| alloc_space | 累计分配字节 | GC CPU 高 |
| alloc_objects | 累计分配次数 | 热点构造 |

**采样机制**：每分配约 `MemProfileRate`（默认 512KB）记录一次栈，小对象可能漏样；调低 rate 更准但更慢。

```mermaid
flowchart LR
  Req[HTTP /debug/pprof/heap] --> GC[可选 gc=1]
  GC --> Snap[堆快照]
  Snap --> Pprof[go tool pprof]
  Pprof --> Top[top / list / web]
```

**常用命令**

```bash
go tool pprof -http=:0 'http://localhost:6060/debug/pprof/heap?gc=1'
go tool pprof -alloc_objects -top http://localhost:6060/debug/pprof/allocs
go tool pprof -base heap1.prof heap2.prof   # 差分
```

**读 top 输出**：flat=自身，cum=子调用含自身；优先看 cum 找入口，flat 找具体分配点。

## 生产场景

- **RSS 缓慢上涨**：每周 heap inuse_space top 类型同一，怀疑 global cache。
- **GC 占比 20%**：allocs 显示 `json.Unmarshal`、`fmt.Sprintf` 顶部。
- **可观测**：定时抓 heap 存 S3，CI 对比回归。

## 排查与工具

| 工具 | 用途 |
|------|------|
| `go tool pprof` | top/list/web/flame |
| `pprof -base` | 版本/时间差分 |
| `runtime/pprof` | 代码内写文件 |
| `GODEBUG=gctrace` | 与 allocs 交叉验证 |

路径：现象 → 选 heap 或 allocs → gc=1 快照 → top -cum → list 函数 → 代码修复 → 复抓对比。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| 常驻 pprof 端口 | 内网/debug 侧车 | 公网暴露 |
| 连续 profiling（Parca/Pyroscope） | 大规模 | 小服务 overhead |
| 压测时抓 profile | 可复现 | 仅生产偶发 |
| 降 MemProfileRate | 精确定位 | 生产长期开 |

## 追问链

1. **heap 与 allocs endpoint 区别？** → 同一 profile 源，默认视图 index 不同。
2. **为何 `?gc=1`？** → 去掉待回收垃圾，inuse 更接近真实存活。
3. **flat 0 cum 高？** → 分配在深层 callee，父函数 cum 高。
4. **采样失真？** → 超大对象必记，小对象低估，需结合 `-m` 与 bench。
5. **生产 overhead？** → 默认很低；rate 过小或频繁 gc=1 会增加成本。

## 反模式与事故

- 只看 alloc 不看 inuse，泄漏未被发现。
- 未 `-base` 对比，把业务增长误判泄漏。
- pprof 端口暴露公网导致信息泄露与 DoS。

## 代码示例

```go
import (
    "net/http"
    _ "net/http/pprof"
    "os"
    "runtime/pprof"
)

func main() {
    go http.ListenAndServe("127.0.0.1:6060", nil)

    f, _ := os.Create("heap.prof")
    defer f.Close()
    runtime.GC()
    _ = pprof.WriteHeapProfile(f)
}
```

## 延伸阅读

- [Diagnostics - The Go Programming Language](https://go.dev/doc/diagnostics)
- [google/pprof README](https://github.com/google/pprof)
- [Profiling Go Programs](https://go.dev/blog/pprof)
