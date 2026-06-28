# Golang 后端面试手册

面向 **5 年+** Go 后端开发者的面试知识库：并发与运行时、内存/GC、系统设计及可运行代码示例。

![gopher](./gopher.png)

## 快速开始

| 步骤 | 链接 |
|------|------|
| 1. 学习路线（4 周 / 8 周） | [docs/learning-path-senior.md](./docs/learning-path-senior.md) |
| 2. 面试题索引 | [docs/interview-catalog.md](./docs/interview-catalog.md) |
| 3. 题单与元数据 | [docs/interview/_meta/questions.yaml](./docs/interview/_meta/questions.yaml) |
| 4. 代码 ↔ 题目映射 | [docs/interview/_meta/mapping.md](./docs/interview/_meta/mapping.md) |

## P0 模块（已发布 55 题）

1. **[并发与运行时](./docs/interview/01-runtime-concurrency/)** — GMP、Channel、Context、泄漏（20 题）
2. **[内存与 GC](./docs/interview/02-memory-gc/)** — 三色标记、逃逸、pprof（15 题）
3. **[系统设计](./docs/interview/03-system-design/)** — 秒杀、幂等、缓存、MQ（20 题）

## 可运行代码

| 目录 | 说明 |
|------|------|
| [basis/](./basis/) | goroutine、channel、sync、struct |
| [gin-example/](./gin-example/) | Gin Web 36 个示例 |
| [gorm/](./gorm/) | GORM、sqlx、事务 |
| [algorithm/](./algorithm/) | LeetCode 参考实现 |
| [examples/senior/](./examples/senior/) | 面试向高级示例（持续补充） |

## 本地运行

```bash
# 阅读文档后，进入对应示例目录
cd basis/goroutine && go run .

# 根模块占位入口
go run .
```

## 静态站点

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements-docs.txt
mkdocs serve   # http://127.0.0.1:8000
```

## 引用来源

见 [docs/sources.md](./docs/sources.md)。

---

原「开发手册」示例代码已保留并映射到面试文档；区块链（ethereum）专题已移除，聚焦通用后端面试。
