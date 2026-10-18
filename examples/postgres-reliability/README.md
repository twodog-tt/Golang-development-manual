# PostgreSQL 资金与消息可靠性演练

对应 [S-PAY-07 账本](../../docs/topics/18-web3-payments-stablecoin/S-PAY-07-postgresql-double-entry-ledger-lab.md)、
[S-DIST-06 Outbox/Inbox](../../docs/topics/middleware/distributed/S-DIST-06-outbox-inbox-fault-lab.md)、
[S-PG-04 恢复](../../docs/topics/middleware/postgresql/S-PG-04-pitr-restore-drill.md)。

使用 SQL/PLpgSQL 实现过账约束和消费原子性，Python 标准库编排 Docker/psql/redis-cli，
避免为教学演练引入额外数据库驱动。需要 Python 3.10+、Docker 和本地镜像：

```bash
docker pull postgres:17-alpine
docker pull redis:7.4-alpine
python3 examples/postgres-reliability/verify.py
```

运行器只检查本地镜像，不自动拉取。`LAB_PG_IMAGE` / `LAB_REDIS_IMAGE` 可固定为兼容的镜像 digest；
本例按 PostgreSQL 17 与 Redis 7.4 编写，报告记录实际服务器版本和本地镜像 ID。

```bash
python3 examples/postgres-reliability/verify.py --only ledger
python3 examples/postgres-reliability/verify.py --only outbox
python3 examples/postgres-reliability/verify.py --only recovery
python3 examples/postgres-reliability/verify.py --report /tmp/knowledge-reliability.json
# 包含 Agent 的整套验收
bash scripts/verify_reliability_examples.sh
```

每次使用唯一容器名称；无宿主端口、无项目目录挂载、无现有数据库连接。初始 trust 认证仅用于这个
未发布端口的短期实验环境，不用于生产配置。正常退出、失败或 Ctrl-C 均清理该次容器和匿名卷。
强制杀死 Python 解释器或 Docker 故障可能阻止清理；可按输出报告中的 run ID 与
`knowledge-reliability-lab` 标签定位该次资源后清理，不应批量删除其他容器。
报告只在整套选定场景通过后写出，命令任一检查失败即非零退出。

| 场景 | 实际验证 |
|------|----------|
| 账本 | 同业务 ID 并发幂等、同键不同意图冲突、跨币种拒绝、并发超扣防护 |
| 冻结/冲正 | 子账户冻结、幂等全额释放、第二次冲正拒绝、不可修改事实 |
| 提交故障 | 过账后 COMMIT 前 `pg_terminate_backend`，余额/journal/Outbox 回滚 |
| 投影差异 | 管理员制造 +1 漂移，检测并在写入隔离下从期初/分录重建 |
| 消息重投 | XADD 后省略标记形成重复 entry；消费回滚、提交后省略 XACK、PEL reclaim |
| Inbox | 去重与投影同事务；相同事件不同 payload 拒绝；最终 event_count=1 |
| 正向恢复 | pg_basebackup + pg_verifybackup + WAL 到命名恢复点，恢复误删前账本 |
| 负向恢复 | 从归档副本删除目标必需 WAL，明确未到达目标且不能提供可写实例 |

Outbox 侧为 `ledger` 数据库，Inbox 侧为另一个 `consumer` 数据库，Broker 为真实 Redis Streams。
为精确复现崩溃窗口，脚本主动省略标记/ACK 后换 worker 身份处理；没有常驻 relay 服务，
也没有声称实际杀死 relay 进程。`XCLAIM idle=0` 仅用于实验，生产需合理租约与 pending 扫描。

余额是期初快照 + 不可变分录的投影；示例采用原子单位整数，按币种守恒。
只支持同币种双边转账、全额反向冲正，未实现完整会计科目、审批、部分退款和跨库资金事务。
Inbox 投影是按币种累计转账量，不能代替资金账本。

恢复源、归档、备份和恢复目录都位于该次容器中，只证明物理恢复机制与业务核验，
不能证明异地容灾或介质损坏恢复。恢复时长是小 fixture 的单次记录，不是生产 RTO。
数据库恢复后还要核对真实支付、链上和消息副作用；本例不会自动重新发出外部付款。

最近一次本地全场景记录见 [local-verification.json](local-verification.json)，含实际版本、镜像 ID、源码 SHA-256 和逐项结果；它是一份可复跑的本地记录，不是外部验收。
