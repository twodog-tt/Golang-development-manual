# Go 后端面试手册

面向 **5 年+** Go 后端开发者的面试知识库。

## 快速导航

- [学习路线（4 周 / 8 周）](learning-path-senior.md)
- [面试题索引](interview/README.md)
- [题单 YAML](interview/_meta/questions.yaml)
- [代码映射](interview/_meta/mapping.md)
- [引用来源](sources.md)

## P0 模块（55 题）

| 模块 | 题数 |
|------|------|
| [01 并发与运行时](interview/01-runtime-concurrency/) | 20 |
| [02 内存与 GC](interview/02-memory-gc/) | 15 |
| [03 系统设计](interview/03-system-design/) | 20 |

## P1 中间件（按类型浏览）

详见 [interview/middleware/](interview/middleware/)：MySQL(5)、Redis(3)、Kafka(1)、RocketMQ(3)、Elasticsearch(3)、分布式事务(1)。

## P1 学习路径模块

| 模块 | 题数 |
|------|------|
| [04 分布式](interview/04-distributed-middleware/) | 入口 → middleware |
| [05 数据库](interview/05-database-storage/) | 入口 → middleware |
| [06 网络](interview/06-network-governance/) | 5 |

## P2 模块（6 题 + 手写代码）

| 模块 | 题数 |
|------|------|
| [07 工程与领导力](interview/07-engineering-leadership/) | 3 |
| [09 云原生](interview/09-cloud-native/) | 3 |
| [08 手写题](interview/08-coding-senior/) | 5（`examples/senior/`） |

**正文合计：87 题**（含中间件专题 RocketMQ×3、ES×3）

## 可运行代码

`basis/` · `gin-example/` · `gorm/` · `algorithm/` · `examples/senior/`
