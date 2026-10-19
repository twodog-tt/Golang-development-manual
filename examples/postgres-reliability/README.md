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
# 无 Docker 的报告生命周期检查
python3 -m unittest discover -s examples/postgres-reliability -p test_verify.py
# 包含 Agent 的整套验收
bash scripts/verify_reliability_examples.sh
```

每次使用唯一容器名称；无宿主端口、无项目目录挂载、无现有数据库连接。初始 trust 认证仅用于这个
未发布端口的短期实验环境，不用于生产配置。正常退出、失败或 Ctrl-C 均清理该次容器和匿名卷。
强制杀死 Python 解释器或 Docker 故障可能阻止清理；可按输出报告中的 run ID 与
`knowledge-reliability-lab` 标签定位该次资源后清理，不应批量删除其他容器。
开始运行即移除同路径的旧报告，自动创建报告父目录。选定场景及清理全部成功后，才通过临时文件 + 原子替换发布成功报告；启动、断言、清理或发布失败均非零退出，不留下旧成功或半写成功报告。报告包含 `selection`，`--only ledger/outbox/recovery` 分别为 8/6/3 个场景，默认完整入口为 **17 个场景**；不要把部分入口的 passed 当作整套通过。

| 场景 | 实际验证 |
|------|----------|
| 账本 | 同业务 ID 并发幂等、同键不同意图冲突、跨币种拒绝、并发超扣防护 |
| 扣款参数 | 直接执行 debit.sql；负数、零、小数、NULL、超额不更新，合法正整数才更新余额/version |
| 冻结/冲正 | 子账户冻结、幂等全额释放、第二次冲正拒绝、不可修改事实 |
| 提交故障 | 过账后 COMMIT 前 `pg_terminate_backend`，余额/journal/Outbox 回滚 |
| COMMIT 回包丢失 | pg_fault.py 经 docker-exec/nc 读取真实 PostgreSQL 协议；拦截 COMMIT 成功回包后只向调用端报告 unknown，独立连接查到已提交，再四路重试原 ID；journal/entries/Outbox 各为 1/2/1 |
| 冲正余额不足 | 收款被转走后冲正拒绝，余额及 journal/entries/Outbox 不留下半成品 |
| 投影差异 | 管理员制造 +1 漂移，检测并在写入隔离下从期初/分录重建 |
| 消息重投 | XADD 后省略标记形成重复 entry；消费回滚、提交后省略 XACK、PEL reclaim |
| Inbox | 去重与投影同事务；相同事件不同 payload 拒绝；最终 event_count=1 |
| Inbox 并发 | 八路同内容只应用一次；两组不同金额争同 ID，只允许一个 payload 胜出，另一组冲突 |
| 已发布消息丢失 | XADD 后标 published，再 XDEL 尚未消费的 entry；用留存 Outbox 与指定消费方 Inbox 核对，原 ID 补投及重复补投后只应用一次 |
| 出款未知 | 独立接收夹具已执行但回执暂不可读，拒绝释放 25 原子单位预占；恢复查询并重试原 ID，外部效果和结算各一次 |
| 正向恢复 | pg_basebackup + pg_verifybackup + WAL 到命名恢复点，恢复误删前账本 |
| 恢复后重放 | PostgreSQL 两库回退、Redis 不回退；隔离恢复副本，核对原始 Outbox/外部回执，重建 Inbox/投影，处理已 ACK 与 PEL 事件；重放提交后丢 ACK 仍能安全重试 |
| 负向恢复 | 从归档副本删除目标必需 WAL，明确未到达目标且不能提供可写实例 |

Outbox 侧为 `ledger` 数据库，Inbox 侧为另一个 `consumer` 数据库，Broker 为真实 Redis Streams。
为精确复现崩溃窗口，脚本主动省略标记/ACK 后换 worker 身份处理；没有常驻 relay 服务，
也没有声称实际杀死 relay 进程。`XCLAIM idle=0` 仅用于实验，生产需合理租约与 pending 扫描。

余额是期初快照 + 不可变分录的投影；示例采用原子单位整数，按币种守恒。
只支持同币种双边转账、全额反向冲正，未实现完整会计科目、审批、部分退款和跨库资金事务。
Inbox 投影是按币种累计转账量，不能代替资金账本。

`debit.sql` 与 PostgreSQL 隔离专题中的扣款语句相同；金额绑定为未舍入 numeric，不能先转整数或 numeric(38,0) 再检查小数。`pg_fault.py` 只为本地 trust 连接制造故障，使用镜像内已有 nc；没有增加 Python 数据库依赖，也不作为通用 PostgreSQL 客户端。

外部接收夹具是 Redis Lua：稳定事件 ID 绑定规范化完整 payload，回执和效果计数原子写入且不设 TTL；相同 ID 不同内容拒绝。它独立于 PostgreSQL PITR，但不是银行/PSP 接口或具备 HA 保证的外部证据系统。出款未知场景使用该夹具验证预占边界，不代表真实出款渠道适配完成。

恢复源、归档、备份和恢复目录都位于该次容器中，只证明物理恢复机制与业务核验，
不能证明异地容灾或介质损坏恢复。恢复时长是小 fixture 的单次记录，不是生产 RTO。
数据库恢复后还要核对真实支付、链上和消息副作用；本例不会自动重新发出外部付款。

`--only recovery` 现在也需要 Redis 镜像。恢复时先保持副本仅暴露容器内 Unix socket，不启动普通 relay/worker；设置 consumer 库 recovery_control.paused 后，普通 consume 和运行器 publisher 拒绝工作。显式操作员入口验证恢复库中 journal/Outbox 与独立接收回执一致，再以原 ID 重放：接收方去重 → recover_consume 原子重建 Inbox/投影/恢复审计 → XACK → 标记恢复库 published。任一步中断都按原 ID/PEL 继续，不重跑资金 transfer。

recover_consume 持控制行锁，只在自身事务内临时解除消费暂停，提交前恢复暂停；其他消费者不能看到这个临时窗口。缺失控制行、原始事实、回执或回执内容不匹配都拒绝执行，不把 SQL NULL 当作放行条件；故障矩阵实际删除控制行并验证普通消费与操作员重放均拒绝且事务回滚。两笔指定事件完成核对、PEL 清零、外部效果各一次、账本数量/余额不变后，才解除本演练的暂停。该门禁只覆盖本地示例消费函数和运行器发布入口；所有连接均为 lab owner，不是生产权限隔离、全局恢复控制器或跨 Redis/PostgreSQL 的 ACID。真实接管必须接入所有在线写入口，并核对恢复点之后的合法业务及外部敞口。

最近一次本地全场景记录见 [local-verification.json](local-verification.json)，含实际版本、镜像 ID、源码 SHA-256 和逐项结果；它是一份可复跑的本地记录，不是外部验收。
