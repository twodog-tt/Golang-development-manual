---
id: S-ES-02
title: Elasticsearch Mapping、查询与聚合
module: elasticsearch
level: senior
frequency: 4
go_version: "1.22+"
tags: [elasticsearch, mapping, query-dsl, aggregation]
status: published
code_refs: []
sources:
  - https://www.elastic.co/guide/en/elasticsearch/reference/current/mapping.html
  - https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html
---

# Elasticsearch Mapping、查询与聚合

## 30 秒版（开场）

> **Mapping** 定义字段类型：`text`（分词全文）vs `keyword`（精确、聚合）；**Query DSL** 组合 `bool`（must/should/filter）；**Aggregation** 做指标与桶统计。生产关键词：**filter 不评分、nested、高基数 cardinality 慎用**。

## 3 分钟版（一面深度）

1. **Mapping**：上线前定好；改字段类型通常需 **reindex**；`dynamic` 可关防字段爆炸。
2. **查询**：`match` 全文；`term` 精确 keyword；`range` 范围；`bool` 组合；**filter context** 可缓存不打分。
3. **聚合**：`terms` 分桶、`date_histogram` 按时间、`avg/sum` 指标；**pipeline agg** 二次计算。

## 10 分钟版（示例）

**典型商品搜索 DSL**

```json
{
  "query": {
    "bool": {
      "must": [{ "match": { "title": "手机" } }],
      "filter": [
        { "term": { "status": "on_sale" } },
        { "range": { "price": { "gte": 1000, "lte": 5000 } } }
      ]
    }
  },
  "sort": [{ "sales": "desc" }],
  "from": 0,
  "size": 20
}
```

| 字段类型 | 用途 | 注意 |
|----------|------|------|
| text | 全文 | 需 analyzer，不宜聚合 |
| keyword | 过滤、排序、聚合 | 不分词 |
| nested | 数组对象独立查询 | 性能成本 |
| join | 父子文档 | 少用，优先宽表 |

**深度分页**

- 避免 `from=10000` → 用 **`search_after`** + sort tiebreaker
- 全量导出 → scroll（旧）/ **PIT + search_after**（新）

## 生产场景

- 搜索推荐：function_score 加权
- 运营报表：terms + sub aggregation
- 日志：date_histogram + 错误率

## 排查与工具

- `_analyze` 看分词结果
- `explain` 看评分
- 慢查询：slowlog

## 架构取舍

| 方案 | 适用 |
|------|------|
| 宽表进 ES | 搜索列表页 |
| MySQL + ES 双写 | 复杂交易仍走 MySQL |
| 仅 keyword 字段 | 不需中文分词时简化 |

## 追问链

1. **text 和 keyword 能否改？** → 一般需 reindex + 新 mapping。
2. **中文分词？** → IK 等插件；索引与搜索 analyzer 可不同。
3. **聚合不准？** → `size` 默认 10；`doc_count_error_upper_bound`；shard 级近似。
4. **Go 客户端？** → `go-elasticsearch` 官方；注意 Bulk 与 context 超时。

## 反模式与事故

- **高基数 terms 聚合**（user_id）→ 内存 OOM
- **wildcard 前导 `*abc`** → 极慢
- **nested 滥用** → 查询复杂、性能差

## 代码示例

```go
// olivere/elastic 风格示意
searchService := client.Search().Index("products").
    Query(elastic.NewBoolQuery().
        Must(elastic.NewMatchQuery("title", "手机")).
        Filter(elastic.NewTermQuery("status", "on_sale")))
```

## 延伸阅读

- [Mapping](https://www.elastic.co/guide/en/elasticsearch/reference/current/mapping.html)
- [Query DSL](https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html)
