#!/usr/bin/env python3
"""从 questions.yaml 生成 interview/.pages 与各模块 .pages，启用题目级（三级）导航。"""

from __future__ import annotations

from collections import defaultdict
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
YAML_PATH = ROOT / "docs/interview/_meta/questions.yaml"
INTERVIEW_DIR = ROOT / "docs/interview"
DOCS_PAGES = ROOT / "docs/.pages"

TIER_KEYS = {"p0", "p1", "p2"}

DIR_TITLES: dict[str, str] = {
    "01-runtime-concurrency": "01 并发与运行时",
    "02-memory-gc": "02 内存与 GC",
    "03-system-design": "03 系统设计",
    "06-network-governance": "06 网络与服务治理",
    "07-engineering-leadership": "07 工程与领导力",
    "08-coding-senior": "08 手写题",
    "09-cloud-native": "09 云原生",
    "10-ai-engineering": "10 AI 工程与编程",
    "11-solution-architecture": "11 解决方案架构",
    "12-blockchain-web3": "12 区块链与 Web3",
    "13-solidity-contracts": "13 Solidity 与合约工程",
    "14-dex-cex-engineering": "14 DEX / CEX 交易所工程",
    "15-microservices-exchange": "15 微服务（交易所场景）",
    "middleware": "中间件与数据库",
    "middleware/mysql": "MySQL",
    "middleware/redis": "Redis",
    "middleware/kafka": "Kafka",
    "middleware/rocketmq": "RocketMQ",
    "middleware/rabbitmq": "RabbitMQ",
    "middleware/elasticsearch": "Elasticsearch",
    "middleware/distributed": "分布式事务",
}

# interview/.pages 二级分组（使用目录 basename，供 awesome-pages 匹配 Section）
INTERVIEW_GROUPS: list[tuple[str, list[str]]] = [
    ("基础 · Go 语言与编码", ["01-runtime-concurrency", "02-memory-gc", "08-coding-senior"]),
    ("进阶 · 网络与中间件", ["06-network-governance", "middleware"]),
    (
        "高阶 · 系统设计与架构",
        ["03-system-design", "09-cloud-native", "11-solution-architecture", "15-microservices-exchange"],
    ),
    ("专题 · Web3 与交易所", ["12-blockchain-web3", "13-solidity-contracts", "14-dex-cex-engineering"]),
    ("综合 · 领导力与 AI", ["07-engineering-leadership", "10-ai-engineering"]),
]

ROOT_PAGES = """nav:
  - 首页: index.md
  - 学习路线: learning-path-senior.md
  - 模拟面试: mock-interview.md
  - interview

  - 参考资料:
    - Web3 交易所重点准备: resume-focus-web3.md
    - 面试题总索引: interview-catalog.md
    - 题源与引用: sources.md
    - 代码与题目映射: interview/_meta/mapping
    - 题目撰写模板: interview/_meta/template
    - 习题质量审查: interview/_meta/quality-review
"""


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
    """侧栏显示题目标题，不显示编号。"""
    return item["title"]


def write_module_pages(rel_dir: str, items: list[dict]) -> None:
    pages_dir = INTERVIEW_DIR / rel_dir
    pages_dir.mkdir(parents=True, exist_ok=True)
    title = DIR_TITLES.get(rel_dir, rel_dir.split("/")[-1])

    lines = [
        f"title: {title}",
        "collapse: false",
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


def write_interview_pages() -> None:
    lines = ["title: 面试专题", "collapse: false", "nav:"]
    for group_title, modules in INTERVIEW_GROUPS:
        lines.append(f"  - {group_title}:")
        for mod in modules:
            lines.append(f"    - {mod}")

    out = INTERVIEW_DIR / ".pages"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {out.relative_to(ROOT)}")


def write_middleware_pages() -> None:
    lines = [
        "title: 中间件与数据库",
        "collapse: false",
        "nav:",
        "  - index.md",
        "  - mysql",
        "  - redis",
        "  - kafka",
        "  - rocketmq",
        "  - rabbitmq",
        "  - elasticsearch",
        "  - distributed",
    ]
    out = INTERVIEW_DIR / "middleware/.pages"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {out.relative_to(ROOT)}")


def write_root_pages() -> None:
    DOCS_PAGES.write_text(ROOT_PAGES + "\n", encoding="utf-8")
    print(f"Wrote {DOCS_PAGES.relative_to(ROOT)}")


def main() -> None:
    data = yaml.safe_load(YAML_PATH.read_text(encoding="utf-8"))
    by_dir: dict[str, list[dict]] = defaultdict(list)

    for item, _chain in iter_questions(data):
        by_dir[str(Path(item["file"]).parent)].append(item)

    write_root_pages()
    write_interview_pages()
    write_middleware_pages()
    for rel_dir in sorted(by_dir):
        write_module_pages(rel_dir, by_dir[rel_dir])

    print(f"Done: {len(by_dir)} module directories")


if __name__ == "__main__":
    main()
