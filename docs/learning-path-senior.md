# 5 年+ Go 后端面试学习路线

> 目标读者：高级工程师 / Tech Lead / Staff 候选  
> 假设：已有 3 年以上 Go 生产经验，需系统补齐**深度原理 + 架构推演 + 领导力表达**

## 能力自检（开始前）

- [ ] 能白板画出 GMP 与 goroutine 生命周期
- [ ] 能读 pprof heap/cpu 并定位泄漏或热点
- [ ] 能在 15 分钟内设计带数字估算的高并发读接口
- [ ] 能讲 2 个真实项目：性能优化、一致性/事故各 1 个
- [ ] 能说明「为什么不用某中间件/某并发模型」

---

## 冲刺版（4 周，在职）

| 周 | 模块 | 阅读 | 练习 | 自测 |
|----|------|------|------|------|
| W1 | [并发与运行时](./interview/01-runtime-concurrency/) | 20 题 | `basis/` + `examples/senior/` | 画 GMP；讲 2 个泄漏案例 |
| W2 | [内存与 GC](./interview/02-memory-gc/) | 15 题 | `go test -race`、`-gcflags=-m` | 读一份 heap profile |
| W3 | [系统设计](./interview/03-system-design/) | 20 题 | 每题 15min 结构化输出 | 秒杀/幂等/缓存各 1 题口述 |
| W4 | P1 模块 + 模拟 | 分布式/DB/网络各选 5 题 | 2 场模拟面 | 追问链不停顿 3 层 |

### 每日建议（工作日 1.5h）

- 40min：精读 2 篇 P0 文档（含追问链）
- 30min：跑/改 1 段关联代码
- 20min：口述「30 秒版 + 1 个生产例子」

---

## 系统版（8 周）

| 周 | 内容 |
|----|------|
| 1-2 | P0：并发 + 内存（同冲刺 W1-W2，加深 pprof/trace） |
| 3-4 | P0：系统设计 20 题 + 容量估算模板 |
| 5 | [分布式与中间件](./interview/04-distributed-middleware/) |
| 6 | [数据库与存储](./interview/05-database-storage/) + `gorm/` |
| 7 | [网络与服务治理](./interview/06-network-governance/) + `gin-example/` |
| 8 | [工程与领导力](./interview/07-engineering-leadership/) + [云原生](./interview/09-cloud-native/) + [手写题](./interview/08-coding-senior/) |

---

## 系统设计答题模板（15 分钟）

```
1. 澄清需求：QPS、读写比、一致性、延迟、可用性
2. 估算：流量、存储、带宽、缓存命中率
3. 架构图：接入层 → 服务层 → 缓存/DB/MQ
4. 核心路径：Happy path + 失败降级
5. 瓶颈与扩展：热点、单点、数据分片
6. 可观测：指标、告警、链路
7. 演进：MVP → 10x → 100x 怎么改
```

---

## 项目故事准备（至少 3 个）

| 类型 | 建议结构 |
|------|----------|
| 性能 | 现象 →  profiling → 改动 → 指标（P99 -X%，CPU -Y%） |
| 一致性/事故 | 触发 → 根因 → 修复 → 预防（规范/演练） |
| 技术决策 | 备选方案 → 权衡矩阵 → 结果与复盘 |

---

## 模块优先级

```
P0（必过）: 01 并发 → 02 内存 → 03 系统设计
P1（大厂二面）: 04 分布式 → 05 数据库 → 06 网络
P2（Lead 面）: 07 工程领导力 → 08 手写题
可选: 09 云原生
```

题单索引：[interview/_meta/questions.yaml](./interview/_meta/questions.yaml)
