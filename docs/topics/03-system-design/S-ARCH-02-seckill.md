---
id: S-ARCH-02
title: 秒杀：库存、超卖、热点 Key
module: system-design
level: senior
frequency: 5
go_version: "1.22+"
tags: [seckill, inventory, hot-key, oversell, redis]
status: published
code_refs: []
sources:
  - https://redis.io/docs/latest/develop/use/patterns/distributed-locks/
---

# 秒杀：库存、超卖、热点 Key

## 30 秒版（开场）

> 秒杀本质是 **有限库存 + 瞬时超高并发**，核心用 **预减库存（Redis Lua）+ 异步下单 + 热点隔离** 防超卖与打穿。生产关键词：**库存扣减原子性、排队削峰、热点 Key 分片**。

## 3 分钟版（精讲深度）

1. **是什么**：限时限量促销，峰值 QPS 可达平时 100~1000 倍，库存通常几百~几万件，参与用户百万级。
2. **为什么**：DB 行锁无法支撑 10 万+ 并发扣库存；热点 SKU 成为 Redis/DB 单点；超卖一次即信任危机。
3. **怎么做**：活动页静态化 + CDN；网关先做身份、限购和 admission control；Redis Lua 原子完成“库存预留 + 用户幂等 + 写入待处理记录”，再由 worker 异步创建订单。Redis 与外部 MQ 之间若双写，必须有可重试的 reservation log 与对账。

## 10 分钟版（原理 + 图示）

**容量估算**

| 维度 | 典型值 |
|------|--------|
| 峰值 QPS | 50,000~200,000（按钮点击） |
| 实际库存 | 1,000 件 |
| 有效成交 | ≤ 1,000（+ 少量待支付超时释放） |
| Redis 单 Key QPS 上限 | ~10 万（需分片或本地预扣） |
| 异步订单写入 | 1,000 TPS 足够 |

```mermaid
flowchart TB
  User[用户] --> CDN[静态活动页]
  User --> GW[网关限流 + 验证码]
  GW --> Lua[Redis Lua 预留库存 + 幂等]
  Lua -->|成功并写 reservation stream| Queue[Redis Stream / Durable Queue]
  Queue --> Worker[秒杀 Worker × N]
  Worker --> OrderMQ[可选订单事件]
  Lua -->|失败| Fail[快速失败]
  Worker --> OrderSvc[订单服务] --> DB[(MySQL)]
  OrderSvc --> Pay[支付超时回滚库存]
```

**防超卖三板斧**

1. **Redis Lua 原子预留**：同一脚本检查库存和用户限购，扣减后写入带 `reservation_id` 的待处理记录，避免“扣了库存但进程在发 MQ 前崩溃”。
2. **权威库存边界**：必须明确 Redis 是 admission/reservation 层还是权威库存。若 DB 仍做条件扣减，按 `reservation_id` 幂等落单并对账；不要把两个独立计数器都叫“强一致库存”。
3. **幂等**：用户+活动维度幂等键，防重复下单。

**热点 Key 治理**

- **库存分片**：可按配额把 token 分到多个 shard，但随机命中空 shard 会产生“假售罄”；需要重试其他 shard、配额再平衡或本地批量领 token，并接受复杂度。
- **请求合并**：网关层令牌桶，每秒只放行库存数倍请求。
- **页面分层**：按钮置灰 + 排队页，减少无效请求。

## 生产场景

- **电商大促秒杀**：iPhone 限量 500 台，500 万 UV，峰值 20 万 QPS。
- **可观测**：Redis 扣减成功率、MQ 堆积、订单创建 TPS、超卖告警（DB stock < 0）。

## 排查与工具

| 工具 | 用途 |
|------|------|
| Redis SLOWLOG / LATENCY | Lua 脚本耗时；避免在繁忙生产实例长期开 `MONITOR` |
| MQ 消费 lag | 订单积压 |
| 业务对账 | Redis 扣减量 vs DB 订单量 |
| 压测 | 模拟 10 万并发验证不超卖 |

路径：用户反馈「抢到了没订单」→ 查 MQ 是否堆积 → Redis 成功但订单失败 → 补偿/回滚库存。

## 架构取舍

| 方案 | 适用 | 不适用 |
|------|------|--------|
| Redis 预扣 + 异步订单 | 高并发秒杀 | 强同步确认（需轮询/WebSocket） |
| DB 悲观锁 | 低并发、强一致 | 万级 QPS |
| 令牌桶排队 | 削峰 | 用户体验要求即时反馈 |
| 分片库存 Key | 单 SKU 热点 | 库存极少（如 10 件）难均分 |

## 深挖问答

1. **Redis 扣了 DB 失败怎么办？** → 定时对账 + 补偿队列；或 TCC 预留库存。
2. **如何防黄牛？** → 验证码、设备指纹、限购、风控规则引擎。
3. **支付超时库存怎么释放？** → 延迟 MQ 检查订单状态，未支付则 INCR 回库存。
4. **Lua 脚本有什么坑？** → 脚本过长阻塞 Redis；大 Key 用分片。
5. **Go Worker 如何扩缩？** → 消费组水平扩展；Redis 集群避免单分片热点。

## 反模式与事故

- 先查库存再扣减（非原子），经典超卖。
- 全量请求直达 DB「保证准确」，DB 宕机。
- 库存 Key 无 TTL，活动结束后 Key 永久残留。
- 未做幂等，用户连点产生多笔订单。

## 代码示例

```go
// 三个 key 使用同一 Redis Cluster hash tag，确保 Lua 可在一个 slot 原子执行。
const reserveStockLua = `
if redis.call('SISMEMBER', KEYS[2], ARGV[1]) == 1 then
  return 2
end
local stock = tonumber(redis.call('GET', KEYS[1]) or '0')
if stock > 0 then
  redis.call('DECR', KEYS[1])
  redis.call('SADD', KEYS[2], ARGV[1])
  redis.call('XADD', KEYS[3], '*',
    'reservation_id', ARGV[2], 'user_id', ARGV[1])
  return 1
end
return 0
`

func (s *SeckillService) TryReserve(
    ctx context.Context, skuID string, userID int64, reservationID string,
) (bool, error) {
    tag := "{" + skuID + "}"
    keys := []string{
        "seckill:" + tag + ":stock",
        "seckill:" + tag + ":users",
        "seckill:" + tag + ":reservations",
    }
    res, err := s.rdb.Eval(
        ctx, reserveStockLua, keys, userID, reservationID,
    ).Int()
    if err != nil {
        return false, err
    }
    return res == 1 || res == 2, nil // 2 表示同一用户已成功预留，幂等返回
}
```

worker 从 Stream/待处理表消费后，以 `reservation_id` 唯一约束创建订单；失败重试，超时 reservation 由补偿任务释放。若改用 Kafka，需保留同等可恢复的本地记录，不能忽略 `Publish` 错误后直接丢失库存。

## 延伸阅读

- [Redis 分布式锁模式](https://redis.io/docs/latest/develop/use/patterns/distributed-locks/)
- [阿里秒杀架构实践](https://developer.aliyun.com/article/1052538)
