---
id: S-CONC-04
title: GOMAXPROCS 调优与容器环境
module: runtime-concurrency
level: senior
frequency: 5
go_version: "1.22+"
tags: [gomaxprocs, kubernetes, cgroup, cpu-quota]
status: published
code_refs:
  - basis/goroutine/main.go
sources:
  - https://go.dev/doc/go1.25
  - https://github.com/uber-go/automaxprocs
  - https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
---

# GOMAXPROCS 调优与容器环境

## 30 秒版（开场）

> **GOMAXPROCS** 决定活跃 P 数，也就是同时执行 Go 代码的并行上限。Go 1.24 及更早
> 语言版本默认通常取 `runtime.NumCPU()`；使用 Go 1.25+ 工具链且模块语言版本为
> `go 1.25` 或更高时，Linux 默认还会考虑 CPU affinity 与 cgroup CPU limit，并可随
> 限制变化自动更新。它不读取 Kubernetes CPU request。不要只看编译器版本，还要看
> `go.mod` 与 `GODEBUG`。

## 3 分钟版（一面深度）

1. **是什么**：设置同时执行 Go 用户代码的最大 P 数；`runtime.GOMAXPROCS(n)` 可动态修改。
2. **为什么**：并行度影响 CPU 利用、GC assist 并行、锁竞争；与 cgroup quota 不一致时，runtime 以为有 32 核实际只有 2 核可用。
3. **怎么做**：裸机常用 runtime 默认；K8s 有 CPU limit 时，使用已启用新默认的 Go
   1.25+ 模块，或在旧语言版本/旧工具链中明确使用 `automaxprocs`。若只配置 request、
   没有 limit，runtime 不会按 request 自动设定。启动日志要记录实际值和配置来源。

## 10 分钟版（原理 + 图示）

```mermaid
flowchart LR
  Host[物理/节点 CPU] --> Cgroup[cgroup CPU quota]
  Cgroup --> Default[逻辑 CPU / affinity / cgroup limit]
  Default --> GMP[GOMAXPROCS = P 数]
  GMP --> Parallel[并行 Go 代码 + GC worker]
```

**历史问题（Go < 1.25）**

- `runtime.NumCPU()` 读的是 **cpuset 可见核**，不是 quota 折算核数。
- 例：limit `200m` 在 8 核节点 → 仍可能 `NumCPU=8`，GOMAXPROCS=8，**严重 cfs throttling**。

**常见策略**

| 环境 | 建议 |
|------|------|
| 裸机/VM 独占 | 默认或略小于核数（留核给系统/网卡） |
| K8s CPU limit | 对齐 quota 核数；Go 1.25+ 新默认需由模块语言版本或显式 `GODEBUG` 启用 |
| limit < 1 CPU | 注意 Go 1.25 默认 **GOMAXPROCS 下限常为 2**（除非可见核/亲和性更低） |
| Burstable 无 limit | runtime 不看 request；如需按 request 控制必须显式配置，并压测验证 |
| 混部 | 显式压低，避免 assist 抢占邻居 |

**Go 1.25**：新默认值通常取逻辑 CPU、CPU affinity 数量与 cgroup 吞吐限额的较小值；
非整数 quota 向上取整，除非逻辑 CPU/affinity 本身小于 2，否则 cgroup 计算不会把默认值
降到 2 以下。为了兼容，模块语言版本 `go 1.24` 及以下默认相当于
`GODEBUG=containermaxprocs=0,updatemaxprocs=0`；仅换成 1.25 工具链不等于已经采用新策略。
手工设置环境变量或调用 `runtime.GOMAXPROCS` 会停用自动更新，可用
`runtime.SetDefaultGOMAXPROCS` 恢复默认策略。

**与 GC**：GOMAXPROCS 会影响并发标记 worker、mutator 与调度开销；调大不保证 STW 更短，在 quota 下还可能增加 throttle 和 P 协调成本，必须看吞吐、P99、GC CPU 与 throttling 实测。

## 生产场景

- **Java 邻居稳定 Go 毛刺**：Go Pod limit 4 核但 GOMAXPROCS=48，P99 周期性与 cgroup 节流对齐。
- **HPA 与节流误判**：cgroup throttle 会改变 CPU 使用、吞吐和尾延迟之间的关系；不能只凭
  “CPU 没到 100%”排除 CPU 配额问题，也不能把节流直接等同于 HPA 失效。
- **可观测**：`container_cpu_cfs_throttled_seconds_total`、`go_sched_gomaxprocs_threads`（自定义暴露 GOMAXPROCS）。

## 排查与工具

```bash
# 容器内
grep Cpus_allowed_list /proc/self/status
cat /sys/fs/cgroup/cpu.max  # cgroup v2
```

- 旧语言版本/旧工具链可评估 `go.uber.org/automaxprocs`。它会显式设置
  `GOMAXPROCS`，因此不要再声称仍享有 Go 1.25 runtime 的动态自动更新。
- trace 看 Proc 利用率是否长期低于预期

## 架构取舍

- **显式配置** vs **automaxprocs**：金融/核心链路倾向启动日志打印最终 GOMAXPROCS。
- **不宜**：为「提高并发」盲目翻倍 GOMAXPROCS；IO 服务瓶颈常在网络而非 P 数。

## 深挖问答

1. **改 GOMAXPROCS 会重启 G 吗？** → 不会杀进程，但调度与 P 池重建，瞬时抖动。
2. **和 worker 池大小关系？** → 独立；worker 是应用层并发，GOMAXPROCS 是 runtime 并行度。
3. **1 核 limit GOMAXPROCS=1 还能并发？** → 能，goroutine 在 IO 等待时让出 P。
4. **NumCPU vs GOMAXPROCS？** → 前者报告硬件/可见核，后者是调度配置，可手动不等。
5. **多容器同节点？** → 按 limit 而非节点核数。

## 反模式与事故

- 镜像未配 automaxprocs，上 K8s 后全面 throttle。
- 压测在 Mac M 系列与生产 Linux cgroup 行为不一致，容量算错。
- `GOMAXPROCS=1` 做「限流」，实际应使用 semaphore/worker 池。

## 代码示例

```go
import (
    "log"
    "runtime"
)

func init() {
    log.Printf("GOMAXPROCS=%d NumCPU=%d", runtime.GOMAXPROCS(0), runtime.NumCPU())
}
```

如果旧模块确实使用 `import _ "go.uber.org/automaxprocs"`，应单独记录该依赖设置的最终值，
并把它视为启动时显式配置，而不是 Go 1.25 的动态默认。

任务并发调度见 [`basis/goroutine/main.go`](https://github.com/twodog-tt/Golang-development-manual/blob/master/basis/goroutine/main.go)。

## 延伸阅读

- [uber-go/automaxprocs](https://github.com/uber-go/automaxprocs)
- [Go 1.25: Container-aware GOMAXPROCS](https://go.dev/doc/go1.25#container-aware-gomaxprocs)
- [Kubernetes CPU limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [Go Wiki: Minimum Requirements](https://go.dev/wiki/MinimumRequirements)
