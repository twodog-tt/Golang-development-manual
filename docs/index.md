# Go 后端面试手册

面向 **5 年+** Go 后端开发者的面试知识库（**103 篇正文**）；含 **架构师岗** 专题。

> **如何使用左侧导航**：点击分组标题（如「中间件与数据库」）可展开/折叠子目录；当前所在分组会自动展开。

## 推荐刷题顺序

1. [学习路线](learning-path-senior.md) — 4 周 / 8 周计划
2. **Go 核心（P0）** — 并发 → 内存 → 系统设计
3. **中间件与数据库** — 按 JD 选 MySQL、Redis、MQ、ES
4. [网络与服务治理](interview/06-network-governance/) — gRPC、Gin、JWT
5. [AI 工程与编程](interview/10-ai-engineering/) — LLM API、RAG、Agent、MCP
6. [**解决方案架构（架构师岗）**](interview/11-solution-architecture/) — DDD、演进、治理、45min 白板
7. [手写题](interview/08-coding-senior/) — LRU、限流等 + `examples/senior/`
8. **工程与软技能** — 领导力、云原生（Lead 面）

## 中间件速查

| 类型 | 题数 | 入口 |
|------|------|------|
| [MySQL + GORM](interview/middleware/mysql/) | 5 | 索引、MVCC、慢查询、分库分表 |
| [Redis](interview/middleware/redis/) | 3 | 集群、分布式锁、热点 Key |
| [Kafka](interview/middleware/kafka/) | 1 | 消费语义 |
| [RocketMQ](interview/middleware/rocketmq/) | 3 | 架构、事务/顺序/延迟 |
| [Elasticsearch](interview/middleware/elasticsearch/) | 3 | 倒排索引、DSL、同步 |
| [分布式事务](interview/middleware/distributed/) | 1 | TCC / Saga |

## 其他链接

- [面试题总索引](interview/README.md)
- [题单 YAML](interview/_meta/questions.yaml)（元数据，非正文）
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)

## 可运行代码

`basis/` · `gin-example/` · `gorm/` · `algorithm/` · `examples/senior/`
