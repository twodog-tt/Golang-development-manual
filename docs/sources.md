# 题源与引用规范

本手册面向 **5 年+ Go 后端**面试准备。正文均为自研表述；外部资料仅作题源与延伸阅读，**不整段搬运**。

## 主要参考来源

| 来源 | 链接 | 用途 |
|------|------|------|
| 2025 GO 开发岗位面试真题分析（168 道） | https://juejin.cn/post/7524308480909344806 | 领域占比、高频标签 |
| 2025 Go 面试八股（100 道含答案） | https://segmentfault.com/a/1190000046610680 | 覆盖面查漏 |
| 大厂 Go 后端 35 道深度解析 | https://developer.cloud.tencent.com/article/2647941 | 追问风格、大厂侧重点 |
| 2024 最全 Go 面经汇总 | https://juejin.cn/post/7434352545870184485 | 真实公司题目 |
| Top 20 Go Interview Questions (uByte) | https://www.ubyte.dev/blog/go-interview-questions | 代码示例结构 |
| Top 50 Go Interview Questions 2026 | https://papersadda.com/article/go-interview-questions-2026/ | 并发与手写题 |
| Go 官方博客 | https://go.dev/blog/ | 版本事实、语言演进 |
| Go Memory Model | https://go.dev/ref/mem | happens-before、并发语义 |
| The Go GC | https://go.dev/blog/ismmkeynote | GC 设计 |
| Scheduler 设计 | https://go.dev/blog/scheduler | GMP 历史 |

## 引用规则

1. 每题 YAML/`sources` 字段至少 1 个外链或官方文档。
2. 博客类内容只链出，正文用自己的话归纳。
3. 标注 `go_version`，避免 1.18 前后泛型、1.22 loop 变量等说法过时。
4. 系统设计题注明假设条件（QPS、一致性级别），便于读者复现推演。

## 版权说明

- 本仓库代码示例遵循项目原有许可。
- 文档内容为学习笔记性质，如有侵权请联系移除。
