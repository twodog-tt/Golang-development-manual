#!/usr/bin/env python3
"""从 questions.yaml 生成 README.md 中的专题全表（序号 + 文档 ID + 标题）。"""

from __future__ import annotations

from collections import defaultdict
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
YAML_PATH = ROOT / "docs/interview/_meta/questions.yaml"
README_PATH = ROOT / "README.md"

TIER_KEYS = {"p0", "p1", "p2"}
MARKER_START = "<!-- QUESTION_TABLE_START -->"
MARKER_END = "<!-- QUESTION_TABLE_END -->"

MODULE_ORDER: list[tuple[str, list[tuple[str, str]]]] = [
    ("基础 · Go 语言与生产工程", [
        ("01 并发与运行时", "concurrency"),
        ("02 内存与 GC", "memory_gc"),
        ("16 Go 生产工程", "go_production_engineering"),
        ("08 编码练习", "coding"),
    ]),
    ("进阶 · 网络与中间件", [
        ("06 网络与服务治理", "network"),
        ("MySQL", "mysql"),
        ("PostgreSQL", "postgresql"),
        ("Redis", "redis"),
        ("Kafka", "kafka"),
        ("RocketMQ", "rocketmq"),
        ("RabbitMQ", "rabbitmq"),
        ("Elasticsearch", "elasticsearch"),
        ("分布式事务", "distributed"),
    ]),
    ("高阶 · 系统设计与架构", [
        ("03 系统设计", "system_design"),
        ("09 云原生", "cloud_native"),
        ("11 解决方案架构", "solution_architecture"),
        ("15 微服务（交易所）", "microservices_exchange"),
    ]),
    ("专题 · Web3 核心基础设施", [
        ("12 区块链与 Web3", "blockchain_web3"),
        ("17 多链钱包与托管", "multichain_wallet"),
        ("18 Web3 支付与稳定币", "web3_payments"),
        ("19 节点、RPC 与 Staking", "node_rpc_staking"),
        ("20 协议、共识与安全", "protocol_consensus_security"),
        ("21 Web3 安全工程", "security_engineering"),
        ("13 Solidity 与合约", "solidity_contracts"),
        ("14 DEX / CEX 交易所", "dex_cex_engineering"),
    ]),
    ("综合 · 领导力与 AI", [
        ("07 工程与领导力", "leadership"),
        ("10 AI 工程", "ai_engineering"),
    ]),
]


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


def escape_table_cell(text: str) -> str:
    return text.replace("|", "\\|")


def build_table(by_key: dict[str, list[dict]]) -> str:
    lines: list[str] = []
    total = sum(len(by_key[k]) for _, mods in MODULE_ORDER for _, k in mods)

    lines.append(MARKER_START)
    lines.append(f"## 专题全表（{total} 篇）")
    lines.append("")
    lines.append(
        "> 序号按 **基础 → 进阶 → 高阶 → 专题 → 综合** 排列；"
        "文档 ID（如 `S-CONC-01`）。点击标题可跳转至 Markdown 正文。"
    )
    lines.append("")

    seq = 0
    for group_name, modules in MODULE_ORDER:
        lines.append(f"### {group_name}")
        lines.append("")
        lines.append("| 序号 | 文档 ID | 标题 |")
        lines.append("|------|------|------|")
        for _mod_label, key in modules:
            for item in by_key.get(key, []):
                seq += 1
                qid = item["id"]
                title = escape_table_cell(item["title"])
                link = f"./docs/interview/{item['file']}"
                lines.append(f"| {seq} | `{qid}` | [{title}]({link}) |")
        lines.append("")

    lines.append(MARKER_END)
    return "\n".join(lines) + "\n"


def patch_readme(table_block: str) -> None:
    readme = README_PATH.read_text(encoding="utf-8")
    if MARKER_START in readme and MARKER_END in readme:
        before = readme.split(MARKER_START)[0]
        after = readme.split(MARKER_END)[1]
        new_readme = before + table_block + after.lstrip("\n")
    else:
        anchor = "## 可运行代码"
        if anchor not in readme:
            raise SystemExit(f"README anchor not found: {anchor}")
        new_readme = readme.replace(anchor, table_block + "\n" + anchor)
    README_PATH.write_text(new_readme, encoding="utf-8")


def main() -> None:
    data = yaml.safe_load(YAML_PATH.read_text(encoding="utf-8"))
    by_key: dict[str, list[dict]] = defaultdict(list)
    for item, chain in iter_questions(data):
        by_key[chain[-1]].append(item)

    table_block = build_table(by_key)
    patch_readme(table_block)
    total = table_block.count("| `S-")
    print(f"Updated {README_PATH.relative_to(ROOT)} ({total} topics)")


if __name__ == "__main__":
    main()
