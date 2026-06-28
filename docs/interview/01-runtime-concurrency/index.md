# 01 并发与运行时

20 题 | P0 | [返回索引](../index.md)

| ID | 题目 | 频率 |
|----|------|------|
| [S-CONC-01](./S-CONC-01-gmp-overview.md) | GMP 模型全貌与抢占式调度 | ⭐⭐⭐⭐⭐ |
| [S-CONC-02](./S-CONC-02-gmp-roles.md) | G、M、P 职责与去掉 P 的后果 | ⭐⭐⭐⭐⭐ |
| [S-CONC-03](./S-CONC-03-goroutine-stack.md) | goroutine 栈与 OS 线程对比 | ⭐⭐⭐⭐ |
| [S-CONC-04](./S-CONC-04-gomaxprocs.md) | GOMAXPROCS 与容器 CPU | ⭐⭐⭐⭐⭐ |
| [S-CONC-05](./S-CONC-05-channel.md) | Channel 底层与选型 | ⭐⭐⭐⭐⭐ |
| [S-CONC-06](./S-CONC-06-channel-deadlock.md) | Channel 死锁 | ⭐⭐⭐⭐⭐ |
| [S-CONC-07](./S-CONC-07-select.md) | select 语义与坑 | ⭐⭐⭐⭐ |
| [S-CONC-08](./S-CONC-08-sync-primitives.md) | Mutex / RWMutex / atomic | ⭐⭐⭐⭐⭐ |
| [S-CONC-09](./S-CONC-09-sync-map.md) | sync.Map | ⭐⭐⭐⭐ |
| [S-CONC-10](./S-CONC-10-sync-pool.md) | sync.Pool | ⭐⭐⭐⭐⭐ |
| [S-CONC-11](./S-CONC-11-waitgroup-once-cond.md) | WaitGroup / Once / Cond | ⭐⭐⭐⭐ |
| [S-CONC-12](./S-CONC-12-context.md) | Context | ⭐⭐⭐⭐⭐ |
| [S-CONC-13](./S-CONC-13-goroutine-leak.md) | goroutine 泄漏 | ⭐⭐⭐⭐⭐ |
| [S-CONC-14](./S-CONC-14-memory-model.md) | 内存模型 | ⭐⭐⭐⭐⭐ |
| [S-CONC-15](./S-CONC-15-race-detector.md) | race detector | ⭐⭐⭐⭐ |
| [S-CONC-16](./S-CONC-16-worker-pool.md) | Worker Pool | ⭐⭐⭐⭐⭐ |
| [S-CONC-17](./S-CONC-17-pipeline.md) | Pipeline / Fan-out | ⭐⭐⭐⭐ |
| [S-CONC-18](./S-CONC-18-goroutine-governance.md) | goroutine 治理 | ⭐⭐⭐⭐ |
| [S-CONC-19](./S-CONC-19-netpoller.md) | netpoller | ⭐⭐⭐⭐ |
| [S-CONC-20](./S-CONC-20-go122-generics.md) | Go 1.22+ 与泛型 | ⭐⭐⭐ |

代码：`basis/goroutine`、`basis/channel`、`basis/sync`
