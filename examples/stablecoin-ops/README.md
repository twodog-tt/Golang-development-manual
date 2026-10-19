# 稳定币支付与资金运营本地项目

四篇专题共用一套代码：[Temporal 资金流程](../../docs/topics/18-web3-payments-stablecoin/S-PAY-08-temporal-money-workflows.md)、[钱包身份](../../docs/topics/18-web3-payments-stablecoin/S-PAY-09-wallet-auth-siwe.md)、[Go/TypeScript 联调](../../docs/topics/18-web3-payments-stablecoin/S-PAY-10-go-typescript-web3-integration.md)、[渠道与运营](../../docs/topics/18-web3-payments-stablecoin/S-PAY-11-stablecoin-channel-operations.md)。

Go API + Temporal Worker + PostgreSQL + viem TypeScript CLI，复用 `examples/postgres-reliability/schema.sql` 的双分录、不可变分录和 Outbox。独立 Go module，不修改根项目 vendor。

```text
预置商户钱包 → SIWE 一次性登录 → 商户作用域支付幂等键
    → Temporal 人工审批/持久计时器 → 模拟渠道提交与状态查询
    → PostgreSQL Inbox → 原子过账/Outbox → 商户 Webhook Inbox
    → 退款工作流 / 异常查询恢复 / 账实漂移检查
```

## 支付到 DeFi 的端到端入口

新增 [完整运行指南](e2e/README.md)：真实本地链收款、索引证据、冻结、V2 兑换、双资产入账、通知和重组冲正。

```bash
python3 examples/stablecoin-ops/e2e/verify.py --report /tmp/payment-e2e/result.json
python3 examples/stablecoin-ops/e2e/verify.py --serve
```

下方是继续保留的模拟渠道模式，其退款、回调、旧历史回放测试与新端到端模式分别说明。

## 一键故障验证

需要 Go 1.24.2+、Node 22、npm、Python 3.10+、Docker。先显式拉取镜像，运行器不会自行拉取：

```bash
docker pull postgres:17-alpine
docker pull temporalio/temporal:1.5.1
python3 examples/stablecoin-ops/verify.py --report /tmp/stablecoin-ops.json
```

运行器使用唯一容器名、随机回环端口、随机本地数据库密码和渠道密钥，不挂载现有数据库或项目目录。报告包含实际 Go/Node 版本、镜像 ID、运行 ID、源文件哈希及逐项结果；任一步失败非零退出且不写成功报告。正常退出、异常和 Ctrl-C 清理本次容器与匿名卷；强杀解释器或 Docker 故障可能留下资源，按 `knowledge-stablecoin-lab=<run_id>` 标签定位，不批量删除其他项目容器。

| 验证 | 判定 |
|---|---|
| SIWE 跨语言 | Go 生成消息，viem 解析重建为完全相同的字节并签名，Go 验签 |
| 身份负例 | 篡改 URI、过期 nonce、同 nonce 并发登录只成功一次；跨商户读请求拒绝 |
| 合约钱包 | Go 单测使用明确的 RPC fixture；固定 chain/block，错误链、错误 magic、RPC 错误拒绝 |
| 精度/幂等 | 大于 JS 安全整数的金额字符串不丢精度；多余小数拒绝；八路并发同意图只生成一个支付 |
| 提交未知 | 渠道已创建但返回 504；Activity 用相同业务 ID 重试，渠道记录仍只有一条 |
| 真实进程故障 | 旧实现夹具 Worker 在等待确认时被 SIGKILL；新 Worker 接续服务器历史 |
| 流程兼容 | 旧夹具无审批审计 marker，新实现通过 GetVersion 保留旧分支；真实历史成功重放，不兼容流程重放明确失败 |
| 回调 | 验证原始 body HMAC、时间窗口、金额/资产/支付身份；重复接受、同 ID 不同内容拒绝、旧序号不覆盖新状态 |
| 提交后响应丢失 | 账本已 COMMIT 后 Activity 失败，重试后仍只有一组 journal/entries |
| 商户通知 | 收件方持久化后返回 504；重投 event ID 不变，Inbox 只有一条；成功后才标记 Outbox published |
| 审批/人工恢复 | 审批超时不创建渠道转账；观察超时进入 review，不自动退款；查询已存在渠道事实可恢复入账 |
| 退款/逆转 | 全额退款提交响应丢失后恢复，反向分录仅一条；已结算 provider reversal 冲正并进入 review_reversed，旧确认不能重新入账 |
| 对账/解绑 | 管理员注入 +1 漂移被检测；解绑撤销商户所有会话和相关 nonce，保留账本事实 |

单独跑纯 Go 测试或 TS 编译：

```bash
cd examples/stablecoin-ops
go test -race ./...
cd client && npm ci --ignore-scripts && npm run build
```

## 交互探索

```bash
# 终端一：启动隔离环境并保留，输出一个临时 lab.env 路径
python3 examples/stablecoin-ops/verify.py --serve
# 终端二：source 输出的完整路径（文件权限 0600，不要提交该文件）
source /实际输出路径/lab.env
cd examples/stablecoin-ops/client
npm run ops -- create order-001 1.000001
npm run ops -- approve PAYMENT_ID
npm run ops -- get PAYMENT_ID
npm run ops -- ops
```

模拟渠道不会自动付款。用该次隔离环境的容器名确认到账，再观察 Worker 入账：

```bash
docker exec "$LAB_PG_CONTAINER" psql -U lab -d lab -c \
  "UPDATE ops.channel SET state='confirmed',sequence=sequence+1 WHERE id='PAYMENT_ID';"
npm run ops -- get PAYMENT_ID
npm run ops -- refund PAYMENT_ID
npm run ops -- ops
# review 状态只查询既存渠道事实，不重建付款
npm run ops -- reconcile PAYMENT_ID
# 已入账、退款或冲正的 Outbox 通知可手工补发
npm run ops -- notify PAYMENT_ID
```

CLI 每次通过本地夹具钱包登录，token 仅留在进程内；服务端只存 token 摘要。夹具私钥 1/2 是公开测试数据，不能充值真实资产。`Asset` 是 chain 31337 上的合成地址，未部署真实 USDC；该独立模拟渠道入口不接链；新增 e2e 入口实际接本地 Anvil 与合成 Token，仍不接公网或真实资金。

## 配置和接口

`DATABASE_URL`、`TEMPORAL_ADDRESS`、`APP_ORIGIN`、`CHANNEL_URL`、`LAB_CHANNEL_SECRET`、`LISTEN_ADDR`；API 默认为回环监听。生产渠道应将 HTTP 适配器替换为明确厂商的 sandbox/生产 API，隔离每个供应商/商户的凭据和签名密钥，并审核状态映射。

登录：`POST /challenge`、`POST /login`；受保护：`POST /payments`、`GET /payments/{id}`、`POST /payments/{id}/{approve,refund,reconcile,notify}`、`GET /ops`、`DELETE /session`、`DELETE /wallets/{address}`。`POST /webhook` 使用渠道 HMAC，不接受商户 bearer 替代。租户来自会话，调用者提供的租户 header 被覆盖。

钱包绑定由初始化 SQL 管理，没有匿名注册或自助新增钱包；解绑会撤销该商户所有会话，最后一个钱包解绑后需管理员恢复。`erc1271` 模式由服务端钱包记录指定，必须配置 `ETH_RPC`；本地验证覆盖模拟 RPC，不声称测试了真实合约钱包。没有实现 ERC-6492、Passkey、完整委托权限或链上权限变化自动撤销会话。

## 证据边界与上线前工作

这是单资产、单模拟渠道、全额退款的集成教学项目；同一 PostgreSQL 中分离了业务、渠道和商户 Inbox 表，不能据此证明跨机房故障、真实 PSP 或生产金融正确性。`lab` 的有限资金期初余额是夹具，不代表资产托管证明；`GET /ops` 对账覆盖账本投影与渠道状态展示，没有接入银行流水。

没有实现少付/多付自动处置、部分退款、多币种 FX、KYC/AML、多人复核、交易签名托管或真实链 finality。生产环境需要独立最小权限数据库角色、HTTPS、受保护的 Temporal namespace、身份限流、角色与审批职责分离、凭据轮换、备份恢复及告警。付款状态未知保留人工处理，不能把超时当作失败后重新付款。退款重试耗尽保留 refund_pending，不自动释放或重发新业务 ID；需核对渠道事实后恢复同一流程。

`local-verification.json` 是一次本地结果，`integration_harness` 只说明可复现集成范围，不是 external acceptance。知识站点仍只发布 Markdown 生成的静态页面，不部署这个支付 API。
