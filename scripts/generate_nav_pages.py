#!/usr/bin/env python3
"""从 questions.yaml 生成 topics/.pages 与各模块 .pages，启用专题级（三级）导航。"""

from __future__ import annotations

from collections import defaultdict
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
YAML_PATH = ROOT / "docs/topics/_meta/questions.yaml"
TOPICS_DIR = ROOT / "docs/topics"
DOCS_PAGES = ROOT / "docs/.pages"

TIER_KEYS = {"p0", "p1", "p2"}

DIR_TITLES: dict[str, str] = {
    "01-runtime-concurrency": "01 并发与运行时",
    "02-memory-gc": "02 内存与 GC",
    "03-system-design": "03 系统设计",
    "06-network-governance": "06 网络与服务治理",
    "07-engineering-leadership": "07 工程与领导力",
    "08-coding-senior": "08 编码练习",
    "09-cloud-native": "09 云原生",
    "10-ai-engineering": "10 AI 工程与编程",
    "11-solution-architecture": "11 解决方案架构",
    "12-blockchain-web3": "12 区块链与 Web3",
    "13-solidity-contracts": "13 Solidity 与合约工程",
    "14-dex-cex-engineering": "14 DEX / CEX 交易所工程",
    "15-microservices-exchange": "15 微服务（交易所场景）",
    "16-go-production-engineering": "16 Go 生产工程",
    "17-multichain-wallet": "17 多链钱包与托管",
    "18-web3-payments-stablecoin": "18 Web3 支付与稳定币",
    "19-node-rpc-staking": "19 节点、RPC 与 Staking",
    "20-protocol-consensus-security": "20 协议、共识与安全",
    "21-security-engineering": "21 Web3 安全工程",
    "middleware": "中间件与数据库",
    "middleware/mysql": "MySQL",
    "middleware/postgresql": "PostgreSQL",
    "middleware/redis": "Redis",
    "middleware/kafka": "Kafka",
    "middleware/rocketmq": "RocketMQ",
    "middleware/rabbitmq": "RabbitMQ",
    "middleware/elasticsearch": "Elasticsearch",
    "middleware/distributed": "分布式事务",
}

# topics/.pages 二级分组（使用目录 basename，供 awesome-pages 匹配 Section）
TOPICS_GROUPS: list[tuple[str, list[str]]] = [
    (
        "基础 · Go 语言与生产工程",
        ["01-runtime-concurrency", "02-memory-gc", "16-go-production-engineering", "08-coding-senior"],
    ),
    ("进阶 · 网络与中间件", ["06-network-governance", "middleware"]),
    (
        "高阶 · 系统设计与架构",
        ["03-system-design", "09-cloud-native", "11-solution-architecture", "15-microservices-exchange"],
    ),
    (
        "专题 · Web3 核心基础设施",
        [
            "12-blockchain-web3",
            "17-multichain-wallet",
            "18-web3-payments-stablecoin",
            "19-node-rpc-staking",
            "20-protocol-consensus-security",
            "21-security-engineering",
            "13-solidity-contracts",
            "14-dex-cex-engineering",
        ],
    ),
    ("综合 · 领导力与 AI", ["07-engineering-leadership", "10-ai-engineering"]),
]

ROOT_PAGES = """nav:
  - 首页: index.md
  - 学习路线: learning-path-senior.md
  - 专题自测: topic-quiz.md

  - 方向与优先级:
    - 角色优先级与证据: topics/_meta/role-priority-matrix
    - P0 知识图谱: topics/_meta/p0-knowledge-graph
    - P0 技术纠错审计: topics/_meta/technical-corrections-audit

  - topics

  - 参考资料:
    - Web3 交易所重点专题: web3-exchange-wallet-focus.md
    - 专题总索引: topic-catalog.md
    - 来源与引用: sources.md
    - 代码与专题映射: topics/_meta/mapping

  - 维护工具:
    - 专题撰写模板: topics/_meta/template
    - 内容质量审查: topics/_meta/quality-review
"""

# 仅影响侧栏展示；正文 H1、搜索索引与专题标题继续使用完整 title。
# 优先覆盖篇数多、术语长、在窄侧栏中容易超过两行的模块。
COMPACT_NAV_TITLES: dict[str, str] = {
    # 12 区块链与 Web3
    "S-BC-11": "Rollup Finality / DA / 强制退出",
    "S-BC-12": "跨链桥认证 / 重放 / 限额",
    # 14 DEX / CEX 交易所工程
    "S-EXCH-01": "CEX 撮合与订单簿",
    "S-EXCH-02": "充值 / 提现 / 钱包体系",
    "S-EXCH-03": "账户与复式记账",
    "S-EXCH-04": "保证金 / 强平 / 资金费率",
    "S-EXCH-05": "风控 / AML / 对账",
    "S-EXCH-06": "DEX AMM 与流动性池",
    "S-EXCH-07": "DEX 聚合 / 滑点 / Gas",
    "S-EXCH-08": "MEV / 抢跑 / 三明治",
    "S-EXCH-09": "CeDeFi 混合架构",
    "S-EXCH-10": "链上成交与 K 线聚合",
    "S-EXCH-11": "WebSocket 行情 Hub",
    "S-EXCH-12": "Token 发行 / 分账 / 返佣",
    "S-EXCH-13": "CEX 端到端架构白板",
    "S-EXCH-14": "Web3 交易所全栈架构",
    "S-EXCH-15": "清结算 / 对账 / 高可用",
    "S-EXCH-16": "永续撮合与仓位引擎",
    "S-EXCH-17": "Go 确定性撮合引擎",
    "S-EXCH-18": "撮合 WAL / 快照 / 回放",
    "S-EXCH-19": "行情序号与 Gap Recovery",
    "S-EXCH-20": "FIX 序号与断线恢复",
    "S-EXCH-21": "STP 自成交防护与监控",
    "S-EXCH-22": "集合竞价与性能验证",
    "S-EXCH-23": "预测市场 CTF 与生命周期",
    "S-EXCH-24": "预测市场 CLOB / EIP-712",
    "S-EXCH-25": "预言机 / 数据源 / 争议",
    "S-EXCH-26": "预测市场安全与上线",
    "S-EXCH-27": "PancakeSwap V2 / V3",
    # 17 多链钱包与托管
    "S-WALLET-01": "Chain Adapter 能力矩阵",
    "S-WALLET-02": "Bitcoin UTXO / PSBT / RBF",
    "S-WALLET-03": "Solana 账户 / PDA / 生命周期",
    "S-WALLET-04": "Cosmos / IBC / Sequence",
    "S-WALLET-05": "Sui Object vs Aptos Resource",
    "S-WALLET-06": "归集 / Nonce / UTXO 恢复",
    "S-WALLET-07": "MPC DKG / Reshare / 恢复",
    "S-WALLET-08": "Solana 离线签名与确认",
    "S-WALLET-09": "Cosmos DIRECT 签名",
    "S-WALLET-10": "Aptos BCS 与执行跟踪",
    "S-WALLET-11": "Sui Object 与能力适配",
    # 19 节点、RPC 与 Staking
    "S-NODE-01": "Ethereum EL / CL 与同步",
    "S-NODE-02": "RPC HA / Quorum / Hedging",
    "S-NODE-03": "Validator / Slashing / 密钥",
    "S-NODE-04": "链数据 Backfill / Trace / Schema",
    "S-NODE-05": "Relayer Nonce / Fee / Finality",
    "S-NODE-06": "节点升级 / 快照 / Pruning",
    "S-NODE-07": "Backfill + Realtime 与 Reorg",
    "S-NODE-08": "Trace / State Diff / Decoder",
    "S-NODE-09": "非 EVM SDK 故障注入与兼容",
    "S-NODE-10": "ClickHouse / Reorg / Lakehouse",
    # 其他超长标题
    "S-NET-06": "Linux FD / epoll / Go netpoll",
    "S-GOENG-03": "Go 单测与 Test Double",
    "S-GOENG-05": "Go Modules 与可复现构建",
    "S-CLOUD-07": "K8s OOM / CrashLoop / Evicted",
    "S-CLOUD-10": "Helm / GitOps / 回滚",
    "S-AI-07": "Go MCP Server",
    "S-AI-08": "多模态与语音接入",
    "S-AI-09": "Agent 工作流 / HITL / 发布",
    "S-AI-10": "Persona / Memory / 反馈治理",
    "S-AI-11": "MCP / A2A 跨框架互操作",
    "S-AI-12": "ERC-8004 身份 / 信誉 / 验证",
    "S-AI-13": "x402 / ERC-8183 Commerce",
    "S-AI-14": "Agent 开放平台 / Launchpad",
    "S-ARCH-21": "CDC / Flink / 实时风控",
    "S-WALLET-12": "TRON / TRC20 交易生命周期",
    "S-PAY-06": "机构托管 / RWA / ISO 20022",
    "S-PROTO-01": "Ethereum PoS 与 Finality",
    "S-PROTO-03": "Blob / DA / PeerDAS",
    "S-SEC-04": "安全测试与事件响应",
    "S-PG-01": "PostgreSQL MVCC / VACUUM",
    "S-PG-03": "PostgreSQL WAL / 复制 / pgx",
    "S-KAFKA-02": "Kafka Producer 可靠性",
}


def iter_questions(node, chain: list[str] | None = None):
    chain = chain or []
    if isinstance(node, list):
        for item in node:
            if isinstance(item, dict) and "id" in item:
                yield item, chain
    elif isinstance(node, dict):
        for key, value in node.items():
            if key in TIER_KEYS:
                yield from iter_questions(value, chain)
            else:
                yield from iter_questions(value, chain + [key])


def yaml_quote(s: str) -> str:
    """YAML 双引号字符串，避免标题含冒号时破坏 awesome-pages 解析。"""
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'


def nav_label(item: dict) -> str:
    """侧栏优先显示紧凑标题，正文和搜索仍保留完整专题标题。"""
    return COMPACT_NAV_TITLES.get(item["id"], item["title"])


def write_module_pages(rel_dir: str, items: list[dict]) -> None:
    pages_dir = TOPICS_DIR / rel_dir
    pages_dir.mkdir(parents=True, exist_ok=True)
    title = DIR_TITLES.get(rel_dir, rel_dir.split("/")[-1])

    lines = [
        f"title: {title}",
        "collapse_single_pages: false",
        "nav:",
    ]
    if (pages_dir / "index.md").exists():
        lines.append("  - index.md")

    for item in items:
        basename = Path(item["file"]).name
        lines.append(f"  - {yaml_quote(nav_label(item))}: {basename}")

    out = pages_dir / ".pages"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {out.relative_to(ROOT)} ({len(items)} questions)")


def write_topics_pages() -> None:
    lines = ["title: 工程专题", "nav:"]
    for group_title, modules in TOPICS_GROUPS:
        lines.append(f"  - {group_title}:")
        for mod in modules:
            lines.append(f"    - {mod}")

    out = TOPICS_DIR / ".pages"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {out.relative_to(ROOT)}")


def write_middleware_pages() -> None:
    lines = [
        "title: 中间件与数据库",
        "nav:",
        "  - index.md",
        "  - mysql",
        "  - postgresql",
        "  - redis",
        "  - kafka",
        "  - rocketmq",
        "  - rabbitmq",
        "  - elasticsearch",
        "  - distributed",
    ]
    out = TOPICS_DIR / "middleware/.pages"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {out.relative_to(ROOT)}")


def write_root_pages() -> None:
    DOCS_PAGES.write_text(ROOT_PAGES.rstrip() + "\n", encoding="utf-8")
    print(f"Wrote {DOCS_PAGES.relative_to(ROOT)}")


def main() -> None:
    data = yaml.safe_load(YAML_PATH.read_text(encoding="utf-8"))
    by_dir: dict[str, list[dict]] = defaultdict(list)
    question_ids: set[str] = set()

    for item, _chain in iter_questions(data):
        question_ids.add(item["id"])
        by_dir[str(Path(item["file"]).parent)].append(item)

    unknown_overrides = sorted(set(COMPACT_NAV_TITLES) - question_ids)
    if unknown_overrides:
        raise SystemExit(
            "Compact nav titles reference unknown IDs: "
            + ", ".join(unknown_overrides)
        )

    write_root_pages()
    write_topics_pages()
    write_middleware_pages()
    for rel_dir in sorted(by_dir):
        write_module_pages(rel_dir, by_dir[rel_dir])

    print(f"Done: {len(by_dir)} module directories")


if __name__ == "__main__":
    main()
