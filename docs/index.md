# Go 后端面试手册

面向 **5 年+** Go 后端开发者的面试知识库。

## 快速导航

- [学习路线（4 周 / 8 周）](learning-path-senior.md)
- [**中间件与数据库（按类型）**](interview/middleware/) ← 推荐
- [面试题总索引](interview/README.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)

## 中间件与数据库（按类型）

左侧导航请点顶部 Tab **「中间件与数据库」**，或点击下方链接：

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](interview/middleware/mysql/) | 5 | 索引、MVCC、慢查询、分库分表、GORM |
| [Redis](interview/middleware/redis/) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](interview/middleware/kafka/) | 1 | 消费语义、Rebalance |
| [RocketMQ](interview/middleware/rocketmq/) | 3 | 架构、事务/顺序/延迟、与 Kafka 对比 |
| [Elasticsearch](interview/middleware/elasticsearch/) | 3 | 倒排索引、DSL、同步运维 |
| [分布式事务](interview/middleware/distributed/) | 1 | TCC / Saga |

## P0 模块（55 题）

| 模块 | 题数 |
|------|------|
| [01 并发与运行时](interview/01-runtime-concurrency/) | 20 |
| [02 内存与 GC](interview/02-memory-gc/) | 15 |
| [03 系统设计](interview/03-system-design/) | 20 |

## 其他 P1 / P2

| 模块 | 说明 |
|------|------|
| [06 网络与服务治理](interview/06-network-governance/) | gRPC、Gin、JWT 等 5 题 |
| [07 工程与领导力](interview/07-engineering-leadership/) | 3 题 |
| [08 手写题](interview/08-coding-senior/) | 5 题 + `examples/senior/` |
| [09 云原生](interview/09-cloud-native/) | 3 题 |

**正文合计：87 题**

## 可运行代码

`basis/` · `gin-example/` · `gorm/` · `algorithm/` · `examples/senior/`
