# 高频必背题单：18 张口述卡

[返回高频题单](high-frequency-roadmap.md) ·
[查看简历定向 P0 图谱](interview/_meta/p0-knowledge-graph.md) ·
[开始模拟面试](mock-interview.md)

这 18 张卡不是新的知识正文，而是把正文压缩成可以闭卷表达的面试答案。每张卡固定回答：
**30 秒结论、3 分钟展开、三个不变量、手画图、项目落点、一个取舍和一个错误表达**。

!!! warning "证据边界"

    “项目落点”只是选择真实案例的提示，不是替你虚构经历。Launchpad 类 DEX、钱包和实时风控
    只能讲本人实际参与的生产部分；OctoAgentFlow 按个人项目或原型证据表达，除非你能提供真实
    生产流量、故障和验收记录，否则不要说成“已在生产验证”。

## 使用方式

1. 第一次只看题目，闭卷讲 30 秒，再展开 3 分钟。
2. 卡住时只看“记忆槽”，不要立刻重读正文。
3. 画图限定 30 秒，图中必须出现状态、边界或数据流，不追求美观。
4. 每次练习都说出“错误表达”，用它阻止自己在压力下说绝对化结论。
5. 当天、1 天、3 天、7 天、14 天、30 天各复述一次；连续两次通过才算掌握。

---

## 第一组：Go 运行时、并发与内存

<a id="s-conc-01"></a>

### 01 · S-CONC-01 GMP 模型与抢占式调度

[查看完整正文](interview/01-runtime-concurrency/S-CONC-01-gmp-overview.md)

!!! abstract "30 秒回答"

    Go 调度器是 G、M、P 组成的 M:N 模型：G 是 goroutine 任务，M 是 OS 线程，P 是执行
    Go 代码所需的调度资源和并行槽位。活跃 P 数由 `GOMAXPROCS` 决定；本地队列、全局队列和
    work stealing 分配 runnable G。阻塞系统调用时 M 可以与 P 解绑，Go 1.14 起的异步抢占则
    改善了纯计算循环长期占用 P，但 Go 调度仍不是硬实时调度。

**3 分钟展开**

1. 先定义 G/M/P，强调“并发 goroutine 数”不等于“并行执行数”。
2. 再讲调度路径：当前 P 本地 runq → 全局 runq → work stealing → netpoll/timer。
3. G 因 channel、锁或网络等待会让出执行；同步 syscall/cgo 可能阻塞 M，runtime 会尽量把 P
   交给其他 M。
4. Go 1.14+ 可在异步安全点抢占长时间运行的 Go 代码；信号、runq 容量等属于版本和平台相关
   的 runtime 实现，不是语言规范。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | M 执行普通 Go 代码需要 P；并行上限约为活跃 P 数；runnable 不等于 running |
| 手画图 | `G → runq(P) → M+P 执行 → waiting/runnable`，旁边补 `syscall: M 阻塞、P handoff` |
| 项目落点 | 用 Launchpad 类 DEX 的链上监听、API 或行情任务说明 IO 等待与 CPU 任务如何隔离；只引用实际 trace/指标 |
| 一个取舍 | CPU 密集任务采用有界 worker 或进程隔离，换取稳定 P99，代价是排队和运维复杂度 |

**错误表达**

- ❌ “一个 P 就是一颗 CPU；Go 每 10ms 必然强制切换 goroutine。”
- ✅ “P 是 runtime 的逻辑调度资源；抢占时机和实现随版本、平台变化，不提供硬实时或严格公平保证。”

**自测追问**：`GOMAXPROCS=1` 是否还有并发？阻塞 syscall、网络 IO 和 cgo 分别怎样影响 M/P？

<a id="s-conc-05"></a>

### 02 · S-CONC-05 Channel 内部实现与选型

[查看完整正文](interview/01-runtime-concurrency/S-CONC-05-channel.md)

!!! abstract "30 秒回答"

    Channel 是带同步语义的类型安全通信原语。当前 runtime 的 `hchan` 主要包含环形缓冲、
    `sendq/recvq` 和锁；无缓冲 channel 需要发送与接收配对，有缓冲 channel 允许生产者最多
    领先 `cap` 个元素。发送成功只证明值已完成交接或进入缓冲，不证明业务处理完成；需要处理
    确认时必须另建 ACK 协议。

**3 分钟展开**

1. 无缓冲适合同步交接；有缓冲适合吸收短时突发，但容量本身就是背压策略。
2. 缓冲满时 send 阻塞，缓冲空时 recv 阻塞；`select` 可以组合取消、超时和降级。
3. 只有明确的发送侧生命周期协调者关闭数据 channel；关闭后接收方可读完缓冲，再得到零值和
   `ok=false`，发送方继续发送会 panic。
4. `len(ch)` 只是瞬时观测，不能拿来做“先检查再发送”的正确性判断；单 channel 多消费者是
   竞争消费，不是消息广播。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | channel 交接不等于处理完成；关闭权必须由协议确定；buffer 不是持久化队列 |
| 手画图 | `producer → [buffer cap=N] → consumer`，满时标 `backpressure`，两侧补 `ctx.Done()` |
| 项目落点 | 链上事件进入有界 channel 后由 worker 落库/写 outbox；说明满队列时阻塞、拒绝还是降级 |
| 一个取舍 | 小缓冲能平滑突发；大缓冲降低短时阻塞，却会放大内存、排队时延并掩盖慢消费者 |

**错误表达**

- ❌ “无缓冲发送返回，说明接收方已经处理完成；关闭 channel 后所有接收都立即返回零值。”
- ✅ “发送返回只保证通信完成；关闭后仍要先消费已有缓冲，业务完成需要独立确认。”

**自测追问**：nil channel 有什么用途？为什么 `close(done)` 能广播结束，而普通业务消息不能自动广播？

<a id="s-conc-08"></a>

### 03 · S-CONC-08 Mutex、RWMutex 与 atomic 选型

[查看完整正文](interview/01-runtime-concurrency/S-CONC-08-sync-primitives.md)

!!! abstract "30 秒回答"

    我按“不变量的范围”选同步原语：复合状态优先用 `Mutex`；`RWMutex` 只在读远多于写且读
    临界区很短时才可能更优；`sync/atomic` 适合单个计数、标志或不可变快照引用。Go 的 atomic
    操作具有顺序一致语义，但多个原子变量并不会自动组成一个原子业务事务，最终仍要用基准和
    mutex/block profile 验证选型。

**3 分钟展开**

1. `Mutex` 适合保护 map、slice 和多字段约束，重点是缩短临界区，不能在持锁时做 RPC。
2. `RWMutex` 在 writer 等待时会阻止新的 reader；它缓解 writer 被持续新读者饿死，但长读仍
   会造成 writer 和后续 reader 排队。
3. atomic 的 Load/Store/Add/Swap/CAS 适合单变量状态；check-then-act 必须放进锁或正确的 CAS
   循环。
4. `atomic.Value` 适合发布不可变配置快照；首次 Store 后具体类型要一致，首次使用后不能复制。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 先定义被保护的不变量；锁内不做慢 IO；单变量原子性不等于多字段一致性 |
| 手画图 | `readers ─RLock→ snapshot` 与 `writer ─Lock→ replace`，再画一条 `atomic.Pointer → immutable config` |
| 项目落点 | 风控/Agent 规则用不可变快照原子发布；订单、余额或返佣等复合状态仍放数据库事务或锁内 |
| 一个取舍 | atomic 减少热点锁竞争，但可读性和正确性证明更难；低竞争路径优先清晰的 Mutex |

**错误表达**

- ❌ “RWMutex 一定比 Mutex 快；用了 atomic 就是 lock-free，而且多字段天然一致。”
- ✅ “是否更快取决于读写比例和临界区；atomic 只保证相应原子操作及其内存序语义。”

**自测追问**：为什么不支持读锁升级写锁？`atomic.Value` 中的对象如果发布后继续被修改会怎样？

<a id="s-conc-12"></a>

### 04 · S-CONC-12 Context 树、取消传播与泄漏

[查看完整正文](interview/01-runtime-concurrency/S-CONC-12-context.md)

!!! abstract "30 秒回答"

    `context.Context` 用于在调用链中传递取消、deadline 和请求域元数据。父取消向子树传播，
    子取消不会反向取消父；`CancelFunc` 只是发出取消并释放关联资源，不等待 goroutine 真正退出。
    下游必须显式监听 `Done`，或使用 `QueryContext`、`NewRequestWithContext` 等支持 context 的
    API，超时才会真正截断工作。

**3 分钟展开**

1. 从请求入口继承框架提供的 ctx，再逐层派生更短的 timeout/deadline，函数第一参数传递。
2. `WithCancel/WithTimeout/WithDeadline` 返回的 cancel 应及时调用，避免 timer 和父子关系长期保留。
3. `Value` 只放 request ID、trace 等请求域数据，key 使用私有类型；业务参数和 client 依赖显式传递。
4. 真正需要脱离父请求的任务可用 `WithoutCancel`，但必须重新设置自己的 deadline、关闭和等待协议。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 取消只向下传播；取消是信号不是等待；超时必须被下游 API/代码消费 |
| 手画图 | `request ctx → DB / RPC / chain RPC`，父节点 `cancel` 向下，子节点不反向 |
| 项目落点 | API → 数据库 → 链 RPC 全程透传 ctx；对账/reconciler 使用服务级生命周期而不是请求 ctx |
| 一个取舍 | 统一 deadline 能及时止损，但层层设置过短会造成无意义失败；要从端到端预算反推各阶段预算 |

**错误表达**

- ❌ “调用 `cancel()` 后所有子 goroutine 已经停止；把 client 放进 `ctx.Value` 比较方便。”
- ✅ “cancel 只关闭取消信号；退出要由下游协作，并由 WaitGroup/errgroup 等机制等待。”

**自测追问**：`Background`、`TODO`、`WithoutCancel` 的区别是什么？循环里为什么不应堆积大量 `defer cancel()`？

<a id="s-conc-13"></a>

### 05 · S-CONC-13 Goroutine 泄漏与 pprof 排查

[查看完整正文](interview/01-runtime-concurrency/S-CONC-13-goroutine-leak.md)

!!! abstract "30 秒回答"

    Goroutine 泄漏是本应结束的 goroutine 因无对端 channel、缺少取消、永久锁等待、无 deadline
    网络调用或后台循环而长期存活。判断不能只看某一时刻的 goroutine 数，而要结合流量归一化
    趋势、两次 profile 的新增栈和业务生命周期。修复的核心是明确 owner、退出信号和等待协议，
    不是依赖 GC 或 `recover`。

**3 分钟展开**

1. 先看 `go_goroutines` 是否在相似负载下持续上升且不回落，再按版本/接口/租户关联变化。
2. 受保护地采集 goroutine profile，间隔一段压测或真实流量后对比，按 `chan send/receive`、
   `select`、`Mutex.Lock`、网络调用等栈聚类。
3. 回到创建点检查：谁启动、谁取消、谁 close、谁 wait、下游是否有 deadline。
4. 修复后做重复启停/压测和泄漏测试；Go 1.26 实验性 leak profile 只能补充识别一类可证明永久
   阻塞的 goroutine，不能替代普通 profile 和指标。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 每个 goroutine 都要有 owner；每条阻塞路径都要有退出条件；profile 必须结合趋势和业务语义 |
| 手画图 | `start → work loop → ctx.Done/chan close → return → Wait`，把缺失的退出边画红叉 |
| 项目落点 | WebSocket/indexer 每连接或每订阅 goroutine：断线 cancel、关闭资源并等待退出；只讲真实排障证据 |
| 一个取舍 | 每连接 goroutine 简单清晰，但连接规模大时资源线性增长；事件循环/worker 池更省资源但实现更复杂 |

**错误表达**

- ❌ “goroutine 数多就是泄漏；pprof 开销很低，可以把调试端点直接暴露公网。”
- ✅ “泄漏由生命周期定义；profile 采集要鉴权、限频，并结合基线和流量判断。”

**自测追问**：正常的高并发阻塞与泄漏怎么区分？如何定位 goroutine 的创建点和无法退出的等待点？

<a id="s-mem-01"></a>

### 06 · S-MEM-01 三色标记与混合写屏障

[查看完整正文](interview/02-memory-gc/S-MEM-01-tri-color-gc.md)

!!! abstract "30 秒回答"

    Go GC 是并发 tracing mark-sweep。三色只是可达性模型：白色尚未发现，灰色已发现但出边
    未扫描完，黑色已扫描完；标记结束仍白的对象才可清扫。并发标记时 mutator 还在修改堆指针，
    因此 Go 1.8+ 的混合写屏障在堆指针覆盖时保护旧值，并在当前 goroutine 栈尚未扫描时保护
    新值，配合逐栈扫描避免标记终止阶段全栈重扫。

**3 分钟展开**

1. 从 roots 扫描得到灰队列，灰对象扫描完变黑，发现的白对象变灰，最后并发 sweep 未标记对象。
2. 若 mutator 在并发标记时新增或删除引用，没有写屏障就可能让仍可达对象漏标。
3. 不要只背“黑不能指白”：Go 的混合屏障维护配套的弱三色不变量；普通栈写不走堆写屏障，
   栈由 GC 逐个短暂停止并扫描。
4. 生产更关注分配率、heap goal、mark assist、GC CPU 和 pause；Go 1.26 默认 Green Tea 改善
   标记/扫描组织与局部性，但没有变成分代或移动压缩 GC。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 只回收不可达对象；屏障保证并发改边不漏标；Go GC 仍有很短的 mark setup/termination STW |
| 手画图 | `roots → gray queue → black`，旁边画 `mutator: heap pointer old/new → write barrier` |
| 项目落点 | JSON/RPC 解码、链上事件聚合的高分配路径：用 alloc/CPU profile 和 trace 证明，再谈复用或数据结构调整 |
| 一个取舍 | 提高 `GOGC` 可降低 GC CPU，却增加堆占用；`GOMEMLIMIT` 是软内存限制目标，不是绝对不会 OOM |

**错误表达**

- ❌ “Go GC 完全并发、没有 STW；混合写屏障就是每次无条件把新旧指针都染灰。”
- ✅ “setup/termination 仍有短 STW；混合屏障对旧值和新值的处理与栈扫描状态有关。”

**自测追问**：mark assist 和写屏障分别解决什么问题？为什么不能把 Green Tea 说成分代 GC？

---

## 第二组：生产后端与系统设计

<a id="s-goeng-01"></a>

### 07 · S-GOENG-01 错误契约、Wrapping 与 Panic 边界

[查看完整正文](interview/16-go-production-engineering/S-GOENG-01-errors-contract-panic-boundary.md)

!!! abstract "30 秒回答"

    Go 的 error 是 API 契约，不只是日志字符串。调用方需要分支处理的错误用稳定 sentinel、
    自定义类型或领域码表达，包装时用 `%w` 保留 cause，再通过 `errors.Is/As` 判断。`panic`
    只适合程序员错误或无法维持的不变量；`recover` 只能在同一 goroutine 的 defer 中生效，
    应放在 request/worker 隔离边界，记录栈并让当前工作单元失败，不能恢复后返回成功。

**3 分钟展开**

1. 先按调用方动作分类：业务拒绝、参数错误、可重试依赖故障、取消/超时和内部 bug。
2. repository/service 保留 cause 并增加 operation 上下文，transport adapter 再映射 HTTP/gRPC
   状态、MQ retry 或 DLQ。
3. 只有愿意把底层错误变成公开兼容契约时才 `%w` 暴露；否则转换成自己的领域错误。
4. 日志由真正处理或丢弃错误的边界记录一次，使用低基数分类码做指标，敏感数据不进入 error。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 错误分类决定调用方动作；包装不能破坏 `Is/As`；panic 后当前工作单元不得假装成功 |
| 手画图 | `repo cause → service classification → HTTP/gRPC/MQ mapping`；旁边画 `panic → boundary → fail closed` |
| 项目落点 | 链 RPC/HSM/发布 API：区分临时不可用、策略拒绝和内部不一致，分别决定重试、拒绝与告警 |
| 一个取舍 | 暴露底层 sentinel 方便调用方判断，却会把实现细节变成长期兼容承诺 |

**错误表达**

- ❌ “所有 error 都 `%w` 并每层打印；recover 后返回 nil，服务就不会崩。”
- ✅ “只暴露稳定契约并在处理边界记录一次；recover 要结束当前工作单元并 fail closed。”

**自测追问**：什么时候不用 `%w`？为什么一个 goroutine 不能 recover 另一个 goroutine 的 panic？

<a id="s-net-07"></a>

### 08 · S-NET-07 TCP 建连、队列与 TIME_WAIT

[查看完整正文](interview/06-network-governance/S-NET-07-tcp-lifecycle-queues-timewait.md)

!!! abstract "30 秒回答"

    TCP 故障不能只看一个 timeout。建连要结合 SYN 处理、已完成握手但尚未 accept 的队列、
    `listen(backlog)`、内核参数和应用 accept/处理速度；连接成功也不代表 TLS 和应用响应正常。
    `TIME_WAIT` 通常在主动关闭方，用于重发最终 ACK 和隔离旧报文，数量多不等于故障，应先看
    连接复用、临时端口、NAT/conntrack 和实际错误率。

**3 分钟展开**

1. 三次握手交换并确认双方初始序列号；Linux 未完成握手和等待 accept 的连接受不同机制约束。
2. 把延迟拆成 DNS、connect、TLS、写请求、TTFB 和读响应，分别设置 timeout 和指标。
3. 重传保证可靠性但增加时延；结合 RTT、重传、队列溢出、FD、端口和应用 P99 交叉定位。
4. 大量短连接优先连接池与 keep-alive；不要为了减少 TIME_WAIT 先改危险复用参数。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | connect 成功不等于请求成功；backlog 不是唯一容量；TIME_WAIT 可能出现在任一主动关闭方 |
| 手画图 | `SYN → SYN-ACK → ACK → accept queue → handler → FIN/ACK → TIME_WAIT` |
| 项目落点 | RPC/WebSocket 网关分别记录 connect、TLS、首包和业务 P99，结合 `ss`、内核统计和抓包定位 |
| 一个取舍 | 长连接减少握手和端口压力，但引入连接保活、负载均衡、重连风暴和资源治理 |

**错误表达**

- ❌ “backlog 就是半连接队列；TIME_WAIT 永远在客户端，调小就能提升性能。”
- ✅ “现代 Linux 的 listen backlog 主要约束等待 accept 队列；TIME_WAIT 由主动关闭角色决定。”

**自测追问**：为什么两次握手不够？TCP keepalive 与 HTTP keep-alive 有什么区别？

<a id="s-db-01"></a>

### 09 · S-DB-01 MySQL 索引与最左前缀

[查看完整正文](interview/middleware/mysql/S-DB-01-mysql-index.md)

!!! abstract "30 秒回答"

    InnoDB 用 B+Tree 聚簇索引组织行，二级索引叶子保存索引列和聚簇索引键；查询列未被覆盖
    时再按聚簇键回表。联合索引 `(a,b,c)` 的普通查找遵循最左前缀，可以利用 `(a)`、
    `(a,b)`、`(a,b,c)`，跳过 `a` 通常不能做普通前缀定位。索引设计必须结合 WHERE、JOIN、
    ORDER BY、回表成本和真实数据分布，用 `EXPLAIN ANALYZE` 验证。

**3 分钟展开**

1. 聚簇索引叶子是完整行；二级索引是独立 B+Tree，因此主键过宽会放大所有二级索引。
2. 联合索引列顺序先匹配查询模式：等值前缀、范围边界和排序，再评估选择性，不是机械地把
   最高选择性列永远放最左。
3. 范围条件后的列通常不能继续缩小扫描区间，但仍可能用于 ICP 或覆盖；MySQL 特定版本还可能
   选择 skip scan，不能把“最左前缀”说成优化器绝无例外。
4. 覆盖索引减少回表，但索引越宽，写放大、缓存占用和 DDL 成本越高。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 索引服务于查询模式；二级索引回表取决于是否覆盖；最终以执行计划和真实分布为准 |
| 手画图 | `secondary B+Tree leaf(index cols + PK) → clustered B+Tree leaf(full row)` |
| 项目落点 | 订单/交易/返佣列表按租户或用户过滤并按时间排序，展示联合索引与 `EXPLAIN ANALYZE` 前后差异 |
| 一个取舍 | 覆盖索引降低读延迟，却增加写放大和存储；高写入表只保留收益可证明的索引 |

**错误表达**

- ❌ “范围条件右边的列全部失效；`SELECT *` 一定回表；选择性最高的列永远放最左。”
- ✅ “区分扫描区间、ICP、覆盖和聚簇访问，并以具体执行计划解释。”

**自测追问**：没有显式主键时 InnoDB 如何选聚簇键？为什么随机宽 UUID 可能增加索引成本？

<a id="s-arch-04"></a>

### 10 · S-ARCH-04 幂等设计

[查看完整正文](interview/03-system-design/S-ARCH-04-idempotency.md)

!!! abstract "30 秒回答"

    幂等不是保证代码只执行一次，而是同一业务意图被重试或重复投递时，不产生第二份可见业务
    效果。API 用“幂等键 + 请求指纹 + 状态 + 首次结果”，MQ 把 inbox/去重记录与业务变更放进
    同一本地事务，数据库再用业务唯一键和条件状态迁移兜底。遇到网络超时的模糊成功，先查询
    权威事实，不能换一个 key 盲目重做。

**3 分钟展开**

1. 幂等键必须标识业务意图并带租户/用户作用域；同 key 不同请求指纹应返回冲突。
2. 持久记录至少区分 PROCESSING、SUCCEEDED、FAILED/UNKNOWN，并处理 owner 崩溃、租约过期和结果保存。
3. 消费者在同一数据库事务内写去重记录和业务结果，成功后才 ack；外部支付、链上交易等仍需
   下游幂等键、状态机和 reconcile。
4. Redis 可以加速热结果或做短期占位，但会过期和丢失，不能替代数据库唯一约束与持久事实。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | key 表示同一意图；key 与请求指纹绑定；去重事实和本地业务变更必须共享提交边界 |
| 手画图 | `request(key+hash) → idempotency record → business TX → saved result`，超时分支指向 `reconcile` |
| 项目落点 | 订单、支付回调、返佣提现或链事件按业务 ID/交易哈希与日志序号幂等；说明 unknown 如何查证 |
| 一个取舍 | 永久保存全部响应最稳但成本高；热结果设 TTL，长期正确性由业务唯一约束和权威状态承担 |

**错误表达**

- ❌ “Redis `SETNX` 成功就保证 exactly-once；接口超时后换 UUID 再请求最安全。”
- ✅ “SETNX 只有缓存/租约语义；模糊成功要沿用同一 intent key 并查询事实。”

**自测追问**：PROCESSING 卡死如何接管？去重表与外部 HTTP 副作用为何不能靠一个本地事务解决？

<a id="s-arch-10"></a>

### 11 · S-ARCH-10 MQ 至少一次、恰好一次与顺序性

[查看完整正文](interview/03-system-design/S-ARCH-10-mq-semantics.md)

!!! abstract "30 秒回答"

    分布式 MQ 最常见的是至少一次：不丢的代价是故障恢复、重试和 rebalance 时可能重复。
    Kafka 的幂等 producer 和事务能在明确的 Kafka read-process-write 边界提供 EOS，但不会把
    外部数据库、支付 API 或链上交易自动纳入事务。业务上的 effect-once 仍依赖 inbox/outbox、
    本地事务、业务幂等和状态机；顺序通常只保证在同一分区或同一业务 key 内。

**3 分钟展开**

1. At-most-once 先提交后处理，可能丢；at-least-once 先处理后提交，可能重复。
2. Kafka producer 幂等消除 broker 重试导致的重复；事务可原子提交 Kafka 写入和消费 offset，
   但外部副作用仍在边界外。
3. 需要同一实体顺序时用稳定 partition key，并在事件中带 version/sequence；全局顺序意味着
   单分区或集中排序，吞吐和可用性代价很高。
4. 重试要分瞬时错误与 poison message，配置退避、上限、DLQ、告警和可审计回放。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 至少一次必然要求消费幂等；顺序范围必须明确；exactly-once 必须声明提交边界 |
| 手画图 | `producer → partition(key) → consumer → local TX(inbox+business) → commit offset` |
| 项目落点 | 链事件到 K 线/风控流水按 token、account 或 order 分区；消费者稳定 ID upsert 并检测 sequence gap |
| 一个取舍 | 增加分区提高吞吐，却削弱跨 key 顺序并增加 rebalance、热点和运维复杂度 |

**错误表达**

- ❌ “Kafka 开启 EOS 后数据库不会重复写；同一个 topic 天然全局有序。”
- ✅ “EOS 只覆盖参与 Kafka 事务的边界；Kafka 的核心顺序保证是分区内顺序。”

**自测追问**：先处理还是先提交 offset？DLQ 为什么不能只是“把失败消息挪走”？

<a id="s-arch-12"></a>

### 12 · S-ARCH-12 支付/订单状态机

[查看完整正文](interview/03-system-design/S-ARCH-12-order-state-machine.md)

!!! abstract "30 秒回答"

    状态机用合法的 `(当前状态, 事件) → 下一状态` 约束订单或支付生命周期。并发回调通过数据库
    条件更新或 version CAS 决定唯一有效迁移，重复事件按业务 ID 幂等；状态变更与 outbox 在
    同一本地事务提交，再异步执行通知等副作用。状态机不能自动回滚已经发生的外部动作，失败时
    要设计新的补偿事件、对账和人工恢复。

**3 分钟展开**

1. 先区分事实状态、命令和事件；终态、可逆迁移和非法迁移必须文档化。
2. 用 `UPDATE ... WHERE status=? AND version=?` 防止支付成功、取消和超时任务互相覆盖。
3. 状态事实与 outbox 原子提交；worker 处理发货、退款、发布等外部副作用，结果再以新事件推进。
4. 记录迁移 actor、reason、event ID、前后状态和版本，监控卡单时长、非法迁移和补偿积压。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 每次迁移必须验证 from-state；重复事件不能重复副作用；补偿是新动作而非时间倒流 |
| 手画图 | `PENDING --pay_success→ PAID --refund_req→ REFUNDING → REFUNDED`，并画 `timeout → CANCELLED` 竞争 |
| 项目落点 | Launchpad 类 DEX 的订单/返佣提现，或 Agent 的 draft→review→execute；明确哪个系统保存权威状态 |
| 一个取舍 | 代码 FSM 简单直接；工作流引擎擅长长流程与定时恢复，但增加运行时、版本和运维成本 |

**错误表达**

- ❌ “有了状态枚举就是状态机；Saga 能自动回滚所有外部操作。”
- ✅ “状态机必须约束迁移和并发；Saga 依赖显式、可能失败的补偿动作。”

**自测追问**：支付成功与用户取消同时到达怎么办？状态更新成功但 MQ 事件没发出去如何恢复？

---

## 第三组：Agent Platform 主线

<a id="s-ai-03"></a>

### 13 · S-AI-03 Agent 与 Function Calling

[查看完整正文](interview/10-ai-engineering/S-AI-03-agent-tool-calling.md)

!!! abstract "30 秒回答"

    Agent 是“模型 + 状态 + 受控工具执行器”的循环，不等于让模型直接操作系统。Function
    Calling 只是让模型提出结构化 tool proposal，名称和参数仍是不可信输入；Go 宿主必须做
    schema、身份、权限、业务状态、幂等和风险审批校验后才能执行。循环还要有 max steps、
    总 deadline、token/成本预算和可审计 trace。

**3 分钟展开**

1. 注册最小、强类型的 tool schema，模型返回 tool call，宿主校验后执行，再把裁剪后的结果回传。
2. schema 合法只代表结构合法，不代表当前用户有权退款、金额合理或资源状态允许。
3. 写工具使用 intent key/状态机；高风险工具走 HITL；无依赖且无冲突的只读工具才能安全并行。
4. 固定流程优先传统状态机或 DAG，只有步骤和工具选择确实需要非确定性决策时才引入 Agent。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 模型只能提议不能授权；工具参数始终不可信；副作用必须有幂等、预算和审计边界 |
| 手画图 | `user → LLM proposal → policy/schema/authz → tool executor → result → LLM` |
| 项目落点 | OctoAgentFlow 讲 tool registry、执行器、step budget 和 policy；按个人项目/原型证据表达 |
| 一个取舍 | 单 Agent + 多工具易治理；多 Agent 可分工，但增加上下文漂移、延迟、成本和授权面 |

**错误表达**

- ❌ “模型输出符合 JSON Schema 就可以执行；用了 ReAct 才叫 Agent。”
- ✅ “Schema 只是第一层校验；Agent 模式不要求固定推理提示，授权永远在确定性系统。”

**自测追问**：并行 tool calls 的安全前提是什么？RAG 与 Agent、MCP 分别解决什么问题？

<a id="s-ai-09"></a>

### 14 · S-AI-09 Agent 工作流、HITL 与可靠发布

[查看完整正文](interview/10-ai-engineering/S-AI-09-agent-workflow-hitl-publishing.md)

!!! abstract "30 秒回答"

    生产 Agent 必须把模型生成和外部副作用分开。模型生成不可变 proposal，审批绑定 proposal
    hash、策略版本和 reviewer；批准后由持久化 execution job 执行。worker 用 lease 加 fencing
    claim 任务，外部调用保存 intent key 和 receipt；网络超时进入 `UNKNOWN`，先查询 provider
    事实再决定重试或人工恢复，不能让模型从头规划并盲目重发。

**3 分钟展开**

1. 决策面产生 draft/proposal 和风险等级；控制面维护 draft→review→approved→executing→终态。
2. Review Queue 管“人批准了什么”，Execution Queue 管“机器执行什么”，两者不能共用一个布尔状态。
3. 审批后执行前重新检查内容版本、权限、OAuth scope、配额和 cooldown；编辑内容使旧审批失效。
4. checkpoint 解决流程恢复，不是外部事实源；HTTP 200/timeout 都要结合 provider object ID、
   查询接口和 receipt 对账。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 审批对象不可静默变化；过期 worker 不能提交状态；UNKNOWN 必须 reconcile 后再重试 |
| 手画图 | `proposal(hash) → review → execution job(lease+epoch) → provider → receipt/UNKNOWN → reconcile` |
| 项目落点 | OctoAgentFlow 讲工作流和 HITL 的设计/原型边界；不要把 checkpoint 设计表述成生产 exactly-once |
| 一个取舍 | 自建 Go 状态机语义清晰；通用工作流引擎恢复能力成熟，但引入新运行时和版本治理 |

**错误表达**

- ❌ “加一个 `approved=true` 就完成 HITL；发布超时直接重试即可。”
- ✅ “审批必须绑定不可变 proposal；模糊成功先对账，只有证明安全时才自动重试。”

**自测追问**：lease 已过期为什么还需要 fencing token？reviewer 编辑内容后如何处理旧审批？

<a id="s-ai-10"></a>

### 15 · S-AI-10 Persona、Memory 与反馈治理

[查看完整正文](interview/10-ai-engineering/S-AI-10-persona-memory-feedback-governance.md)

!!! abstract "30 秒回答"

    Agent Memory 不是把全部聊天历史塞进 prompt。我会分开版本化 Persona/Policy、线程短期状态、
    长期事实与偏好、RAG 内容和反馈学习规则，并按 tenant/bot/user/scene 做 namespace、权限、
    TTL 和来源治理。检索必须先做授权过滤再排序；一次 run 保存实际使用的版本和 memory snapshot。
    用户反馈先成为 candidate，经证据聚合、审核和灰度后才可变成 active rule。

**3 分钟展开**

1. Thread/checkpoint 用于续跑；长期 memory 跨会话但会过期；账户余额和授权仍要实时查询权威系统。
2. Persona 是目标、语气和边界的版本化配置，普通对话不能静默提升权限或覆盖合规策略。
3. Context assembly 按权限、任务、freshness、来源、冲突和 token budget 选择片段；向量相似度只是候选信号。
4. 反馈规则经历 candidate→approved→active→retired，保存阈值算法、证据和 policy version，
   不能把模型自报 confidence 当校准概率。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | Memory、Context、RAG 不等价；授权过滤发生在检索前；高价值事实必须有权威来源和版本 |
| 手画图 | `namespace + policy + thread + retrieval → context assembler → model → run snapshot`；反馈走独立规则状态机 |
| 项目落点 | OctoAgentFlow 讲 Persona、thread state、memory namespace 和反馈规则；只陈述真实实现层级 |
| 一个取舍 | 全量历史实现快但贵且污染；结构化 memory 可治理，却需要 schema、失效、删除和迁移机制 |

**错误表达**

- ❌ “向量库就是 Memory；top-k 相似就可以跨租户召回；66% confidence 是模型可信概率。”
- ✅ “向量索引只是召回手段；权限、版本、来源和阈值定义必须由系统治理。”

**自测追问**：用户说“以后都自动发布”能否写入 Persona？删除用户数据时缓存和向量索引怎么办？

<a id="s-ai-05"></a>

### 16 · S-AI-05 LLM 应用安全

[查看完整正文](interview/10-ai-engineering/S-AI-05-llm-security.md)

!!! abstract "30 秒回答"

    LLM 安全不只是内容过滤。用户输入、RAG 文档、网页和模型输出都视为不可信数据；Prompt
    Injection 检测无法提供绝对隔离，所以真正的权限必须由服务端 IAM、最小 tool 能力、参数和
    业务校验、高风险确认、输出按目标 sink 编码以及资源预算共同保证。密钥不进入 prompt，
    PII 按最小必要、授权、脱敏、驻留和保留策略治理。

**3 分钟展开**

1. 威胁覆盖直接/间接注入、敏感信息泄露、不安全输出处理、过度代理权限和无界资源消耗。
2. 文档来源、隐藏文本扫描和注入分类器只能降低风险，不能证明模型永远不听恶意指令。
3. Tool executor 继承真实用户/服务身份，做 allowlist、RBAC/ABAC、最小 scope、幂等、配额和审批。
4. 模型输出进入 SQL、HTML、shell 或 URL 时，按各自 sink 参数化/编码；trace/prompt 默认不记录敏感正文。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 模型不是安全边界；授权在代码层；输入、检索结果和输出都属于不可信数据 |
| 手画图 | `untrusted input/RAG → model → untrusted output → policy+authz → sandboxed tool/sink` |
| 项目落点 | Agent 工具权限与 Launchpad/钱包签名权限类比：模型可提议，策略服务和 signer 才能授权 |
| 一个取舍 | 私有化模型改善数据驻留，但不能自动解决注入、越权、供应链和错误输出 |

**错误表达**

- ❌ “system prompt 写了禁止越权就安全；做一次关键词过滤就能防 Prompt Injection。”
- ✅ “Prompt 层只能降低风险，损害上限由最小权限、确定性校验和隔离控制。”

**自测追问**：Prompt Injection 与普通输入注入有何不同？为什么输出过滤不能替代 tool authorization？

<a id="s-ai-06"></a>

### 17 · S-AI-06 LLM 可观测性、成本与延迟

[查看完整正文](interview/10-ai-engineering/S-AI-06-llm-observability-cost.md)

!!! abstract "30 秒回答"

    LLM 服务除了 RED 指标，还要观测 TTFT、总时延、输入/输出及 provider 返回的其他 token 用量、
    重试、tool steps、任务质量和单位任务成本。一次 Agent run 要串起 RAG、模型和工具 span，
    但 prompt、tool 参数和结果默认不全量记录。缓存和模型路由必须经过评估；semantic cache
    还要把租户、权限、prompt/model 版本和数据时效放进隔离边界。

**3 分钟展开**

1. 区分模型调用与整个 run：单次 latency 低，不代表十步 Agent 的总时延和成本可接受。
2. 记录 provider/model/version、TTFT、总时延、token、重试、错误、cache hit 和估算/账单成本，
   质量用离线集、人工反馈和任务完成率共同评价。
3. 优化顺序是先减少无效步骤和上下文，再考虑 prompt caching、semantic cache、模型路由、batch
   和 streaming；每种手段都有质量、时效和安全边界。
4. OpenTelemetry GenAI 约定仍在演进和迁移，项目要锁定 semconv/instrumentation 版本，不把高基数
   prompt 正文当 span attribute。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | 成本按完整任务统计；优化不能越过质量门槛；缓存 key 必须包含授权与版本边界 |
| 手画图 | `request → RAG → LLM₁ → tool → LLM₂ → output`，每段标 latency/token/cost/quality |
| 项目落点 | OctoAgentFlow 展示预算、step 上限和 trace 设计；无生产账单时只说测试数据和观测方案 |
| 一个取舍 | 小模型/缓存降低成本和 TTFT，但可能损失质量、时效或隔离性，必须离线+灰度验证 |

**错误表达**

- ❌ “streaming 降低了模型总计算时间；语义相似的回答可以跨用户直接复用。”
- ✅ “streaming 主要改善首屏体验；缓存必须满足租户、权限、版本和时效约束。”

**自测追问**：TTFT、TPOT 和端到端时延分别说明什么？失败重试为什么会造成隐蔽成本放大？

<a id="s-arch-21"></a>

### 18 · S-ARCH-21 CDC、Flink、ES 与可重放风控链路

[查看完整正文](interview/03-system-design/S-ARCH-21-realtime-risk-cdc-flink.md)

!!! abstract "30 秒回答"

    实时风控链路要把 OLTP 事实、CDC 日志、Flink 状态、在线特征和 ES 检索投影分层。CDC 用
    一致性 snapshot 与 source position 衔接增量；Flink 处理 keyed state、event time、watermark
    和 checkpoint；sink 用稳定 event/document ID、版本或事务提交。Flink exactly-once 首先是
    故障恢复后每条事件只影响托管状态一次，不自动保证 ES 或外部动作端到端只发生一次。

**3 分钟展开**

1. 事件契约携带稳定 event ID、source position、主键、op、schema version 和 event time；
   `before/after` 完整性取决于数据库与 connector 配置。
2. Kafka/raw lake 保留可重放事实；Flink 用 watermark 处理乱序和迟到，watermark 是进度估计，
   不是“之后绝无迟到”。
3. 端到端 effect-once 需要可重放 source、checkpointed offset、确定性状态和事务或幂等 sink；
   ES bulk 还必须逐 item 检查结果。
4. 升级先回放到 shadow feature/index，对数量、内容、延迟和决策结果做对账后切 alias；回放数据
   不能再次触发封禁、通知、扣款等命令。

| 记忆槽 | 内容 |
|--------|------|
| 三个不变量 | OLTP/领域事件是事实，ES 是可重建投影；event ID 必须稳定；回放数据与外部命令隔离 |
| 手画图 | `OLTP → CDC → Kafka/raw → Flink → Redis/ES → Go risk API`，旁边画 `replay → shadow → compare → switch` |
| 项目落点 | 出行平台实时风控可讲真实数据契约、Go 服务、SLO 和排障边界；没写过 Flink 算子就不声称内核开发 |
| 一个取舍 | CDC 适合通用数据投影；领域 outbox 更能表达业务意图，但需要业务代码改造和 relay 运维 |

**错误表达**

- ❌ “开启 Flink EXACTLY_ONCE 后 ES 永不重复；watermark 到了就不会再有迟到事件。”
- ✅ “checkpoint 与 sink 语义必须分别说明；watermark 是可配置的事件时间进度估计。”

**自测追问**：snapshot 与增量如何避免中间漏数？为什么回放只能重建数据状态，不能重放外部命令？

---

## 通过标准

每张卡满足以下条件才算完成：

- 不看正文能在 30 秒内给出结论、范围和失败边界；
- 3 分钟回答按因果展开，不堆术语；
- 30 秒内画出卡片中的状态或数据流；
- 项目案例能说清本人职责、约束、指标口径和证据等级；
- 主动纠正卡片列出的错误表达；
- 面试官打断后，能从任一“记忆槽”继续回答，而不是从头背稿。
