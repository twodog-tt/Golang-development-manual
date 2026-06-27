# Go 后端面试手册

面向 **5 年+** Go 后端开发者的面试知识库：原理、生产实践、系统设计与追问链。

## 快速开始

1. 阅读 [学习路线（4 周 / 8 周）](./learning-path-senior.md)
2. 按 P0 优先级刷题：[并发](./interview/01-runtime-concurrency/) → [内存/GC](./interview/02-memory-gc/) → [系统设计](./interview/03-system-design/)
3. 对照 [题单](./interview/_meta/questions.yaml) 与 [代码映射](./interview/_meta/mapping.md) 运行示例

## 目录

| 模块 | 说明 | 状态 |
|------|------|------|
| [01 并发与运行时](./interview/01-runtime-concurrency/) | GMP、Channel、Context、泄漏 | P0 已发布 20 题 |
| [02 内存与 GC](./interview/02-memory-gc/) | GC、逃逸、slice/map、pprof | P0 已发布 15 题 |
| [03 系统设计](./interview/03-system-design/) | 高并发、缓存、MQ、多活 | P0 已发布 20 题 |
| [**中间件按类型**](./interview/middleware/) | MySQL、Redis、Kafka、RocketMQ、ES | **19 题** |
| [04 分布式](./interview/04-distributed-middleware/) | 学习路径入口 | → middleware |
| [05 数据库](./interview/05-database-storage/) | 学习路径入口 | → middleware |
| [06 网络治理](./interview/06-network-governance/) | gRPC、Gin、JWT | P1 已发布 5 题 |
| [07 工程领导力](./interview/07-engineering-leadership/) | 事故、技术债、带团队 | P2 已发布 3 题 |
| [08 手写题](./interview/08-coding-senior/) | LRU、限流、优雅退出 | 代码已发布 |
| [09 云原生](./interview/09-cloud-native/) | K8s、Docker、OTel | P2 已发布 3 题 |

## 可运行代码

```
basis/          # Go 并发、channel、sync 基础示例
gin-example/    # Gin Web 36 个示例
gorm/           # GORM、sqlx 示例
algorithm/      # LeetCode 算法
examples/senior/ # 面试向 senior 示例（持续补充）
```

## 静态站点（MkDocs）

本地预览（需 Python 3.10+）：

```bash
python3 -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -r requirements-docs.txt
mkdocs serve
# 浏览器打开 http://127.0.0.1:8000 — 支持全文搜索与侧边栏导航
```

构建静态文件：`mkdocs build` → 输出到 `site/`

## 引用与题源

见 [sources.md](./sources.md)。新增题目请复制 [template.md](./interview/_meta/template.md)。
