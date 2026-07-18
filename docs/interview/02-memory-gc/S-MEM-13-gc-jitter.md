---
id: S-MEM-13
title: 延迟敏感服务的 GC 抖动治理
module: memory-gc
level: senior
frequency: 5
go_version: "1.22+"
tags: [gc-jitter, latency, p99, realtime]
status: published
code_refs: []
sources:
  - https://go.dev/doc/gc-guide
  - https://go.dev/doc/go1.19
  - https://go.dev/doc/go1.26
  - https://github.com/golang/proposal/blob/master/design/44167-gc-pacer-redesign.md
---

# 延迟敏感服务的 GC 抖动治理

## 30 秒版（开场）

> **GC 抖动** 指 P99/P999 延迟被 STW、mark assist、sweep 争抢打断；治理需 **降存活集、平滑分配、GOMEMLIMIT、版本升级**，而非简单堆内存「越大越好」。生产关键词：**pause 分位、assist 毛刺、与 CPU throttle 叠加**。

## 3 分钟版（一面深度）

1. **是什么**：并发 GC 仍会在分配尖峰触发 assist，或在 sweep term 产生短 STW，表现为尾延迟尖刺。
2. **为什么**：延迟敏感服务（交易、游戏、广告竞价）SLO 在毫秒级，亚毫秒 STW 叠加队列即超时。
3. **怎么做**：压测看 `/gc/pauses:seconds` 与请求尾延迟；trace 对齐尖刺；控制分配 burst；合理设置 memory limit。调 GOGC 的方向必须通过 CPU、RSS、assist 和 P99 一起验证。

## 10 分钟版（原理 + 图示）

**抖动来源**

| 来源 | 表现 |
|------|------|
| Mark termination STW | 短而尖 |
| Mark assist | 请求线程变慢，无 STW 但 P99↑ |
| Sweep 争用 | 分配路径与 sweep 竞争 |
| 堆突增 | 促销/批任务导致 GC 连环触发 |
| CPU limit | GC 与业务抢 cgroup 份额 |

```mermaid
sequenceDiagram
  participant R as Request
  participant M as Mutator
  participant GC as GC
  R->>M: 处理
  M->>M: assist 标记
  Note over M: P99 拉长
  GC->>M: STW term
  Note over R: 超时
```

**pacer（1.19+ 改进）**：根据 `GOMEMLIMIT` 与 GOGC 动态调节触发点，减少「堆暴涨后连环 GC」。

**Go 1.26 Green Tea**：默认收集器通过更好的标记/扫描局部性降低不少 GC-heavy 工作负载的 CPU 开销，但收益并非所有负载相同，也不代表 STW、assist 或尾延迟自动按同一比例下降。延迟服务仍必须用自己的对象图、CPU 型号和限额配置做升级前后对照。

**治理 playbook**

1. 基线：固定 RPS 压测，记录 P50/P99/P999 与 `/gc/pauses:seconds`。
2. trace 5s 捕获尖刺时刻是否 STW/assist。
3. 降 burst 分配（批处理改队列平滑）。
4. 根据进程内非 Go 内存和峰值留 headroom 设置 `GOMEMLIMIT`；围绕默认 GOGC 做多组实验，不预设“50~80 一定更稳”。
5. 升级 Go 小版本获 pacer/GC 修复。

## 生产场景

- **竞价 RTB**：100ms 超时，P999 偶发 8ms → trace 见 2ms STW + assist。
- **FinTech 撮合**：GC 与业务同核，cgroup 1 CPU，assist 与 throttle 双重恶化。
- **可观测**：histogram `go_gc_duration_seconds`、自定义「请求耗时 − 业务耗时」残差。

## 排查与工具

| 工具 | 用途 |
|------|------|
| `go tool trace` | 尖刺对齐 STW |
| `runtime/metrics` | pause 分位 |
| 压测 + 限流 | 复现 burst |

路径：P99 破 SLO → trace → 若 GC → allocs/存活/GOGC/GOMEMLIMIT → 平滑负载或隔离。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| 分配隔离（批处理另进程） | 混合负载 | 运维复杂 |
| 略降 GOGC | 需要压低堆峰值、CPU 有余，且压测证实尾延迟改善 | CPU 已紧或 assist/GC 频率反而恶化 |
| 超大堆换少 GC | 吞吐批处理 | 延迟敏感 |
| 非 Go 处理极热路径 | 极端 | 过早 |

## 追问链

1. **assist 如何体现在 trace？** → mutator 时间片执行 `gcAssistAlloc`。
2. **GOMEMLIMIT 如何影响抖动？** → 合理值可约束 runtime 内存并降低撞 cgroup limit 的风险；设得过低会 GC thrashing，反而放大抖动。
3. **能否禁用 GC？** → `GOGC=off` 只关闭堆增长触发，有限 GOMEMLIMIT 仍可触发；仅适合生命周期和峰值都可证明的特殊任务。
4. **和 netpoller 延迟区分？** → trace network vs GC 视图。
5. **SLO 多少 GC 可接受？** → 没有通用阈值，应从端到端预算倒推，并同时计入 STW、assist、排队与 CPU throttle。

## 反模式与事故

- 促销倍流量无预热，分配 burst 触发连环 GC，全站 P99 爆。
- 只扩容 Pod 不控分配，堆线性涨，mark 时间随存活集增。
- 用 sleep 错开 GC——不可靠，应靠 pacer 与架构。

## 代码示例

```go
// 平滑 burst：有界队列 + 固定 worker，避免瞬时百万分配
func startSmoothedWorkers(n int, q <-chan Job) {
    var wg sync.WaitGroup
    for i := 0; i < n; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            buf := make([]byte, 0, 4096) // worker 级复用
            for j := range q {
                buf = buf[:0]
                process(j, &buf)
            }
        }()
    }
    wg.Wait()
}
```

## 延伸阅读

- [Go GC Guide - Latency](https://go.dev/doc/gc-guide)
- [GC Pacer Redesign（proposal）](https://github.com/golang/proposal/blob/master/design/44167-gc-pacer-redesign.md)
- [Go 1.19 GOMEMLIMIT](https://go.dev/doc/go1.19)
- [Go 1.26 Release Notes - Green Tea GC](https://go.dev/doc/go1.26)
