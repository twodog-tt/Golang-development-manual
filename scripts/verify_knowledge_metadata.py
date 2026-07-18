#!/usr/bin/env python3
"""校验题单、角色化优先级、证据标签与正文的最小一致性。"""

from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any, Iterable

import yaml

ROOT = Path(__file__).resolve().parents[1]
QUESTIONS_PATH = ROOT / "docs/interview/_meta/questions.yaml"
ROLE_EVIDENCE_PATH = ROOT / "docs/interview/_meta/role-evidence.yaml"
INTERVIEW_DIR = ROOT / "docs/interview"
TIER_KEYS = {"p0", "p1", "p2"}


def iter_questions(
    node: Any,
    tier: str | None = None,
    chain: tuple[str, ...] = (),
) -> Iterable[tuple[str | None, tuple[str, ...], dict[str, Any]]]:
    if isinstance(node, list):
        for item in node:
            if isinstance(item, dict) and "id" in item:
                yield tier, chain, item
        return
    if not isinstance(node, dict):
        return
    for key, value in node.items():
        if key in TIER_KEYS:
            yield from iter_questions(value, key, chain)
        else:
            yield from iter_questions(value, tier, chain + (key,))


def frontmatter(path: Path) -> dict[str, Any]:
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---\n"):
        raise ValueError(f"{path.relative_to(ROOT)} 缺少 YAML frontmatter")
    parts = text.split("---\n", 2)
    if len(parts) != 3:
        raise ValueError(f"{path.relative_to(ROOT)} frontmatter 未闭合")
    data = yaml.safe_load(parts[1]) or {}
    if not isinstance(data, dict):
        raise ValueError(f"{path.relative_to(ROOT)} frontmatter 不是对象")
    return data


def check_unique(name: str, values: list[str], errors: list[str]) -> None:
    duplicates = sorted(value for value, count in Counter(values).items() if count > 1)
    if duplicates:
        errors.append(f"{name} 存在重复 ID: {', '.join(duplicates)}")


def check_ids(
    name: str,
    values: list[str],
    known_ids: set[str],
    errors: list[str],
) -> None:
    check_unique(name, values, errors)
    unknown = sorted(set(values) - known_ids)
    if unknown:
        errors.append(f"{name} 引用了未知 ID: {', '.join(unknown)}")


def main() -> None:
    questions = yaml.safe_load(QUESTIONS_PATH.read_text(encoding="utf-8"))
    metadata = yaml.safe_load(ROLE_EVIDENCE_PATH.read_text(encoding="utf-8"))
    rows = list(iter_questions(questions))
    errors: list[str] = []

    ids = [item["id"] for _, _, item in rows]
    check_unique("questions.yaml", ids, errors)
    known_ids = set(ids)
    published_ids = {
        item["id"] for _, _, item in rows if item.get("status") == "published"
    }

    docs: dict[str, dict[str, Any]] = {}
    for tier, chain, item in rows:
        qid = item["id"]
        if tier not in TIER_KEYS:
            errors.append(f"{qid} 缺少历史 tier")
        if not chain:
            errors.append(f"{qid} 缺少 module chain")
        file_value = item.get("file")
        if not file_value:
            errors.append(f"{qid} 缺少 file")
            continue
        path = INTERVIEW_DIR / file_value
        if not path.is_file():
            errors.append(f"{qid} 正文不存在: {path.relative_to(ROOT)}")
            continue
        try:
            fm = frontmatter(path)
        except ValueError as exc:
            errors.append(str(exc))
            continue
        docs[qid] = fm
        if fm.get("id") != qid:
            errors.append(f"{qid} 与正文 frontmatter id={fm.get('id')} 不一致")
        if fm.get("status") != item.get("status"):
            errors.append(f"{qid} 的 questions.yaml/status 与正文不一致")
        if not fm.get("sources"):
            errors.append(f"{qid} 缺少 sources，不能标记 source_anchored")
        text = path.read_text(encoding="utf-8")
        if "## 30 秒版" not in text:
            errors.append(f"{qid} 缺少 30 秒版")
        if "## 追问链" not in text:
            errors.append(f"{qid} 缺少追问链")

    shared_p0 = list(metadata["shared"].get("p0", []))
    shared_p1 = list(metadata["shared"].get("p1", []))
    check_ids("shared.p0", shared_p0, known_ids, errors)
    check_ids("shared.p1", shared_p1, known_ids, errors)
    overlap = sorted(set(shared_p0) & set(shared_p1))
    if overlap:
        errors.append(f"shared.p0/p1 重叠: {', '.join(overlap)}")

    role_counts: dict[str, tuple[int, int, int]] = {}
    role_labels: dict[str, str] = {}
    for role_key, role in metadata.get("roles", {}).items():
        role_p0 = list(role.get("p0", []))
        role_p1 = list(role.get("p1", []))
        check_ids(f"roles.{role_key}.p0", role_p0, known_ids, errors)
        check_ids(f"roles.{role_key}.p1", role_p1, known_ids, errors)
        overlap = sorted(set(role_p0) & set(role_p1))
        if overlap:
            errors.append(
                f"roles.{role_key}.p0/p1 重叠: {', '.join(overlap)}"
            )
        effective_p0 = (set(shared_p0) | set(role_p0)) & published_ids
        effective_p1 = (
            (set(shared_p1) | set(role_p1)) - effective_p0
        ) & published_ids
        effective_p2 = published_ids - effective_p0 - effective_p1
        role_counts[role_key] = (
            len(effective_p0),
            len(effective_p1),
            len(effective_p2),
        )
        role_labels[role_key] = role.get("label", role_key)

    evidence = metadata.get("evidence", {})
    reproducibility = evidence.get("reproducibility", {})
    evidence_levels = [
        "deterministic_test",
        "integration_harness",
        "external_acceptance",
    ]
    evidence_sets: dict[str, set[str]] = {}
    for level in evidence_levels:
        values = list(reproducibility.get(level, []))
        check_ids(f"evidence.reproducibility.{level}", values, known_ids, errors)
        evidence_sets[level] = set(values)
    for index, left in enumerate(evidence_levels):
        for right in evidence_levels[index + 1 :]:
            overlap = sorted(evidence_sets[left] & evidence_sets[right])
            if overlap:
                errors.append(
                    f"evidence {left}/{right} 重叠: {', '.join(overlap)}"
                )

    volatility = evidence.get("volatility", {})
    for key in ("version_sensitive", "vendor_or_regulatory_sensitive"):
        values = list(volatility.get(key, []))
        check_ids(f"evidence.volatility.{key}", values, known_ids, errors)

    experience_ids = list(metadata.get("experience_pattern", []))
    check_ids("experience_pattern", experience_ids, known_ids, errors)

    evidence_counts: Counter[str] = Counter()
    volatility_counts: Counter[str] = Counter()
    version_ids = set(volatility.get("version_sensitive", []))
    vendor_ids = set(volatility.get("vendor_or_regulatory_sensitive", []))
    version_prefixes = tuple(volatility.get("version_sensitive_prefixes", []))
    external_ids = evidence_sets["external_acceptance"]
    integration_ids = evidence_sets["integration_harness"]
    deterministic_ids = evidence_sets["deterministic_test"]

    for qid in published_ids:
        fm = docs.get(qid, {})
        if qid in external_ids:
            evidence_counts["external_acceptance"] += 1
        elif qid in integration_ids:
            evidence_counts["integration_harness"] += 1
        elif qid in deterministic_ids:
            evidence_counts["deterministic_test"] += 1
        elif fm.get("code_refs"):
            evidence_counts["illustrative_artifact"] += 1
        else:
            evidence_counts["explanation_only"] += 1

        if qid in vendor_ids:
            volatility_counts["vendor_or_regulatory_sensitive"] += 1
        elif qid in version_ids or qid.startswith(version_prefixes):
            volatility_counts["version_sensitive"] += 1
        else:
            volatility_counts["stable"] += 1

    if errors:
        print("Knowledge metadata validation failed:")
        for error in errors:
            print(f"- {error}")
        raise SystemExit(1)

    print(
        f"OK: {len(ids)} questions, {len(published_ids)} published, "
        f"{len(docs)} article files"
    )
    print("Role priorities:")
    for role_key, counts in role_counts.items():
        print(
            f"- {role_labels[role_key]} ({role_key}): "
            f"P0={counts[0]} P1={counts[1]} P2={counts[2]}"
        )
    print("Reproducibility evidence:")
    for level in reproducibility.get("precedence", []):
        print(f"- {level}: {evidence_counts[level]}")
    print("Volatility:")
    for level in (
        "stable",
        "version_sensitive",
        "vendor_or_regulatory_sensitive",
    ):
        print(f"- {level}: {volatility_counts[level]}")
    print(f"Experience-pattern articles: {len(experience_ids)}")


if __name__ == "__main__":
    main()
