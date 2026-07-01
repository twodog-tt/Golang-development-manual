#!/usr/bin/env python3
"""从 questions.yaml + 题目 Markdown 生成模拟面试用的 questions.json。"""

from __future__ import annotations

import json
import re
from datetime import datetime, timezone
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
YAML_PATH = ROOT / "docs/interview/_meta/questions.yaml"
INTERVIEW_DIR = ROOT / "docs/interview"
OUT_PATH = ROOT / "docs/data/questions.json"

MODULE_LABELS: dict[str, str] = {
    "concurrency": "01 并发与运行时",
    "memory_gc": "02 内存与 GC",
    "system_design": "03 系统设计",
    "mysql": "MySQL",
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
    "leadership": "07 工程与领导力",
    "cloud_native": "09 云原生",
    "coding": "08 手写题",
    "solution_architecture": "11 解决方案架构",
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


def mkdocs_url(file_rel: str) -> str:
    path = file_rel.removesuffix(".md")
    return f"interview/{path}/"


def main() -> None:
    data = yaml.safe_load(YAML_PATH.read_text(encoding="utf-8"))
    questions = []

    for item, chain in iter_questions(data):
        if item.get("status") != "published":
            continue
        file_rel = item["file"]
        md_path = INTERVIEW_DIR / file_rel
        prompt = extract_prompt(md_path) if md_path.is_file() else ""
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
            }
        )

    questions.sort(key=lambda q: q["id"])
    payload = {
        "version": 1,
        "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "count": len(questions),
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
