#!/usr/bin/env python3
"""把 docs/topics 下正文映射为 interview → topics 的 mkdocs-redirects 条目。"""

from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TOPICS_DIR = ROOT / "docs/topics"
MKDOCS = ROOT / "mkdocs.yml"
START = "        # AUTO-REDIRECTS-START"
END = "        # AUTO-REDIRECTS-END"


def build_block() -> str:
    lines = [
        START,
        "        interview-catalog.md: topic-catalog.md",
    ]
    for md in sorted(TOPICS_DIR.rglob("*.md")):
        rel = md.relative_to(TOPICS_DIR).as_posix()
        lines.append(f"        interview/{rel}: topics/{rel}")
    lines.append(END)
    return "\n".join(lines)


def main() -> None:
    text = MKDOCS.read_text(encoding="utf-8")
    if START not in text or END not in text:
        raise SystemExit("mkdocs.yml missing AUTO-REDIRECTS markers")
    before, rest = text.split(START, 1)
    _, after = rest.split(END, 1)
    MKDOCS.write_text(before + build_block() + after, encoding="utf-8")
    count = build_block().count("interview/")
    print(f"Updated redirect maps in mkdocs.yml ({count} interview/* entries)")


if __name__ == "__main__":
    main()
