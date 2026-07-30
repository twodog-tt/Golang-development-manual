#!/usr/bin/env python3
"""从 questions.yaml + 专题 Markdown 生成专题自测用的 questions.json。"""

from __future__ import annotations

import json
import re
from datetime import datetime, timezone
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
YAML_PATH = ROOT / "docs/topics/_meta/questions.yaml"
ROLE_EVIDENCE_PATH = ROOT / "docs/topics/_meta/role-evidence.yaml"
TOPICS_DIR = ROOT / "docs/topics"
OUT_PATH = ROOT / "docs/data/questions.json"

MODULE_LABELS: dict[str, str] = {
    "concurrency": "01 并发与运行时",
    "memory_gc": "02 内存与 GC",
    "system_design": "03 系统设计",
    "mysql": "MySQL",
    "postgresql": "PostgreSQL",
    "redis": "Redis",
    "kafka": "Kafka",
    "rocketmq": "RocketMQ",
    "rabbitmq": "RabbitMQ",
    "elasticsearch": "Elasticsearch",
    "distributed": "分布式事务",
    "network": "06 网络与服务治理",
    "ai_engineering": "10 AI 工程",
    "blockchain_web3": "12 区块链与 Web3",
    "solidity_contracts": "13 Solidity 与合约",
    "dex_cex_engineering": "14 DEX / CEX",
    "microservices_exchange": "15 微服务（交易所）",
    "leadership": "07 工程与领导力",
    "cloud_native": "09 云原生",
    "coding": "08 编码练习",
    "solution_architecture": "11 解决方案架构",
    "go_production_engineering": "16 Go 生产工程",
    "multichain_wallet": "17 多链钱包与托管",
    "web3_payments": "18 Web3 支付与稳定币",
    "node_rpc_staking": "19 节点、RPC 与 Staking",
    "protocol_consensus_security": "20 协议、共识与安全",
    "security_engineering": "21 Web3 安全工程",
}

TIER_KEYS = {"p0", "p1", "p2"}


def module_label(chain: list[str]) -> str:
    if not chain:
        return "其他"
    if chain[0] == "middleware" and len(chain) > 1:
        return MODULE_LABELS.get(chain[-1], chain[-1])
    return MODULE_LABELS.get(chain[-1], chain[-1])


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


def extract_prompt(md_path: Path) -> str:
    text = md_path.read_text(encoding="utf-8")
    match = re.search(
        r"## 30 秒版[^\n]*\n+(?:> ?(.+?)(?:\n|$))+(?=\n## |\Z)",
        text,
        re.MULTILINE,
    )
    if not match:
        return ""
    block = re.search(
        r"## 30 秒版[^\n]*\n+((?:>[^\n]*\n?)+)",
        text,
    )
    if not block:
        return ""
    lines = []
    for line in block.group(1).splitlines():
        line = line.strip()
        if line.startswith(">"):
            lines.append(line[1:].strip())
    return " ".join(lines)


def extract_frontmatter(md_path: Path) -> dict:
    text = md_path.read_text(encoding="utf-8")
    if not text.startswith("---\n"):
        return {}
    parts = text.split("---\n", 2)
    if len(parts) != 3:
        return {}
    data = yaml.safe_load(parts[1]) or {}
    return data if isinstance(data, dict) else {}


def build_role_priorities(config: dict, question_ids: set[str]) -> dict[str, dict[str, str]]:
    shared_p0 = set(config["shared"].get("p0", []))
    shared_p1 = set(config["shared"].get("p1", []))
    priorities: dict[str, dict[str, str]] = {qid: {} for qid in question_ids}
    for role_key, role in config.get("roles", {}).items():
        effective_p0 = shared_p0 | set(role.get("p0", []))
        effective_p1 = (shared_p1 | set(role.get("p1", []))) - effective_p0
        for qid in question_ids:
            if qid in effective_p0:
                priority = "p0"
            elif qid in effective_p1:
                priority = "p1"
            else:
                priority = config.get("default_priority", "p2")
            priorities[qid][role_key] = priority
    return priorities


def evidence_for(qid: str, frontmatter: dict, config: dict) -> dict[str, str]:
    evidence = config["evidence"]
    reproducibility = evidence["reproducibility"]
    if qid in set(reproducibility.get("external_acceptance", [])):
        reproducibility_label = "external_acceptance"
    elif qid in set(reproducibility.get("integration_harness", [])):
        reproducibility_label = "integration_harness"
    elif qid in set(reproducibility.get("deterministic_test", [])):
        reproducibility_label = "deterministic_test"
    elif frontmatter.get("code_refs"):
        reproducibility_label = "illustrative_artifact"
    else:
        reproducibility_label = reproducibility.get("default", "explanation_only")

    volatility = evidence["volatility"]
    vendor_ids = set(volatility.get("vendor_or_regulatory_sensitive", []))
    version_ids = set(volatility.get("version_sensitive", []))
    version_prefixes = tuple(volatility.get("version_sensitive_prefixes", []))
    if qid in vendor_ids:
        volatility_label = "vendor_or_regulatory_sensitive"
    elif qid in version_ids or qid.startswith(version_prefixes):
        volatility_label = "version_sensitive"
    else:
        volatility_label = volatility.get("default", "stable")

    basis = (
        "experience_pattern"
        if qid in set(config.get("experience_pattern", []))
        else evidence["basis"].get("default", "source_anchored")
    )
    return {
        "basis": basis,
        "reproducibility": reproducibility_label,
        "volatility": volatility_label,
        "reviewed_at": config["updated_at"],
    }


def mkdocs_url(file_rel: str) -> str:
    path = file_rel.removesuffix(".md")
    return f"topics/{path}/"


def main() -> None:
    data = yaml.safe_load(YAML_PATH.read_text(encoding="utf-8"))
    role_config = yaml.safe_load(ROLE_EVIDENCE_PATH.read_text(encoding="utf-8"))
    all_rows = list(iter_questions(data))
    all_ids = {item["id"] for item, _chain in all_rows}
    role_priorities = build_role_priorities(role_config, all_ids)
    questions = []

    for item, chain in all_rows:
        if item.get("status") != "published":
            continue
        file_rel = item["file"]
        md_path = TOPICS_DIR / file_rel
        prompt = extract_prompt(md_path) if md_path.is_file() else ""
        article_frontmatter = extract_frontmatter(md_path) if md_path.is_file() else {}
        questions.append(
            {
                "id": item["id"],
                "title": item["title"],
                "frequency": int(item.get("frequency", 3)),
                "module": module_label(chain),
                "module_key": chain[-1] if chain else "",
                "resume_focus": bool(item.get("resume_focus")),
                "url": mkdocs_url(file_rel),
                "prompt": prompt or item["title"],
                "role_priorities": role_priorities[item["id"]],
                "evidence": evidence_for(
                    item["id"], article_frontmatter, role_config
                ),
            }
        )

    questions.sort(key=lambda q: q["id"])
    payload = {
        "version": 2,
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "count": len(questions),
        "roles": [
            {
                "key": role_key,
                "label": role.get("label", role_key),
                "objective": role.get("objective", ""),
            }
            for role_key, role in role_config.get("roles", {}).items()
        ],
        "questions": questions,
    }
    OUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    OUT_PATH.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"Wrote {len(questions)} questions to {OUT_PATH.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
