"""Presentation data for the library theme; Markdown remains the content source."""
from __future__ import annotations

from pathlib import Path
import re

import yaml
from mkdocs.structure.toc import AnchorLink


def _topics(node):
    if isinstance(node, list):
        yield from (item for item in node if isinstance(item, dict) and "id" in item)
    elif isinstance(node, dict):
        for value in node.values():
            yield from _topics(value)


def on_config(config):
    root = Path(config.config_file_path).parent
    source = root / "docs/topics/_meta/topics.yaml"
    topics = {item["id"]: item for item in _topics(yaml.safe_load(source.read_text()))
              if item.get("status") == "published"}
    evidence = yaml.safe_load((source.parent / "role-evidence.yaml").read_text())
    config.extra["library"] = {
        "count": len(topics),
        "reviewed_at": evidence.get("reviewed_at", evidence["updated_at"]),
        "topics": topics,
        "featured": [
            {"id": qid, "label": label, "description": description}
            for qid, label, description in [
                ("S-NODE-11", "Web3", "持久签名、Nonce 提价与进程中断恢复。"),
                ("S-NODE-12", "Go", "快照分页、日志质量门禁与链重组回放。"),
                ("S-EXCH-32", "Web3", "真实本地 Uniswap V2 报价、授权与执行核对。"),
                ("S-PAY-08", "Web3", "审批、持久计时器、Worker 恢复与历史回放。"),
            ] if qid in topics
        ],
    }
    return config


def on_env(env, config, files):
    def nav_title(value):
        return re.sub(r"^\d{2}\s+(?!分钟)", "", str(value)).split("：", 1)[0]
    env.filters["library_nav_title"] = nav_title
    return env


def on_page_content(html, page, config, files):
    if not page.meta.get("id"):
        return html
    # Keep the Markdown heading, anchor and TOC intact. Only attach presentation
    # classes and split its editable text; no article is rewritten on disk.
    heading = re.search(r"<h1(?P<attrs>[^>]*)>(?P<text>.*?)</h1>", html, re.S)
    if heading:
        text = heading["text"]
        text, separator, subtitle = text.partition("：")
        if separator:
            replacement = (f'<h1{heading["attrs"]} class="library-article-title">'
                           f'<span class="library-title-main">{text}</span>'
                           f'<span class="library-title-subtitle"><span class="library-title-separator">：</span>{subtitle}</span></h1>')
        else:
            replacement = f'<h1{heading["attrs"]} class="library-article-title">{text}</h1>'
        html = html[:heading.start()] + replacement + html[heading.end():]
    # The initial summary reads as a quiet editorial callout, regardless of
    # whether the existing source uses a blockquote or an admonition.
    html = re.sub(r'(<h2[^>]*>30 秒版.*?</h2>)\s*<blockquote>',
                  r'<blockquote class="library-summary">\1', html, count=1, flags=re.S)
    return html


def on_page_context(context, page, config, nav):
    library = config.extra["library"]
    if page.is_homepage:
        page.toc.items[:] = [AnchorLink("工程知识库", "go-agent-web3", 1)]
    module_index = next((p for p in nav.pages if p.url == str(Path(page.url).parent) + "/"), None)
    qid = page.meta.get("id")
    context["library_article"] = bool(qid)
    context["library_module"] = module_index
    context["library_reviewed_at"] = str(page.meta.get("reviewed_at", library["reviewed_at"]))
    context["library_tag_labels"] = {
        "ledger": "资金账本", "postgresql": "PostgreSQL", "double-entry": "双分录",
        "idempotency": "幂等", "reconciliation": "对账", "agent": "Agent",
        "evaluation": "评测", "regression": "回归", "safety": "安全",
    }

    # Reuse the existing MkDocs tree and its active states, regrouped into the
    # three shelves in the selected design. Every published page remains linked.
    tree = next((item for item in nav.items if item.title == "工程专题"), None)
    if tree:
        go, ai, web3 = [], [], []
        for group in tree.children:
            if "Web3 核心基础设施" in group.title:
                web3.extend(group.children)
            elif "领导力与 AI" in group.title:
                for child in group.children:
                    (ai if "AI" in child.title else go).append(child)
            else:
                go.append(group)
        context["library_shelves"] = [
            {"title": title, "children": children, "active": any(item.active for item in children)}
            for title, children in [("Go 生产工程", go), ("AI Agent 工程", ai), ("Web3 基础设施", web3)]
        ]
    else:
        context["library_shelves"] = []

    featured = []
    for item in library["featured"]:
        topic = library["topics"][item["id"]]
        target = next((p for p in nav.pages if p.file.src_uri == "topics/" + topic["file"]), None)
        if target:
            featured.append({**item, "url": target.url, "title": ("Agent 评测与发布门禁" if item["id"] == "S-AI-15" else topic["title"].split("：", 1)[0]),
                             "date": str(target.meta.get("reviewed_at", library["reviewed_at"]))})
    context["library_featured"] = featured
    return context
