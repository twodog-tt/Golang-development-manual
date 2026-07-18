---
id: S-MODULE-NNN
title: 题目标题
module: runtime-concurrency | memory-gc | system-design | distributed | database | postgresql | network | leadership | coding | cloud-native | ai-engineering | solution-architecture | blockchain-web3 | solidity-contracts | dex-cex-engineering | go-production-engineering | multichain-wallet | web3-payments-stablecoin | node-rpc-staking | protocol-consensus-security | security-engineering
level: senior
frequency: 5
go_version: "1.22+"
tags: []
status: draft
resume_focus: false
code_refs: []
sources: []
---

# 题目标题

> 新增题目后还要更新
> [questions.yaml](./questions.yaml) 与
> [role-evidence.yaml](./role-evidence.yaml)。岗位 P0/P1/P2 和证据标签以中央元数据为准，
> 不在每篇正文重复维护；没有真实测试、localnet、硬件或生产验收时，不得写成“已验证”。

## 30 秒版（开场）

> 一句话结论 + 一个生产关键词。

## 3 分钟版（一面深度）

1. **是什么**：定义与边界
2. **为什么**：设计动机
3. **怎么做**：关键机制或步骤

## 10 分钟版（原理 + 图示）

> 可配合 `docs/assets/` 中的 Mermaid 图；讲清数据结构与流程。

## 生产场景

- 典型业务场景
- 可观测指标（QPS、P99、goroutine 数、GC pause 等）
- 常见故障现象

## 排查与工具

- 使用的工具（pprof、trace、metrics、日志）
- 排查路径（从现象到根因）

## 架构取舍

- 方案 A vs B 的适用条件
- 何时**不**使用该方案

## 追问链

1. 第一层追问 → 简答要点
2. 第二层追问 → 简答要点
3. 第三层追问 → 简答要点

## 反模式与事故

- 典型误用
- 线上教训（可匿名）

## 代码示例

```go
// 链接 examples/senior/ 或 basis/ 等
```

## 延伸阅读

- [标题](https://example.com)

## 发布前自检

- [ ] 30 秒版先说范围、不变量和失败边界，没有绝对化结论
- [ ] 追问链至少覆盖一次“为什么不选另一个方案”
- [ ] `sources` 优先官方规范、源代码或产品文档，并标明版本敏感点
- [ ] SQL/配置/代码片段与目标版本一致
- [ ] 有 `code_refs` 只代表关联 artifact；测试/harness/外部验收按中央证据标签声明
- [ ] Staff/项目案例中的数字来自本人真实证据，模板数字使用占位符
