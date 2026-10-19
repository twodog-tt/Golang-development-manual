"""Local intent-v1 only. No external packages, no normalization of Unicode."""
import hashlib
import json
import sys


def pairs(items):
    result = {}
    for key, value in items:
        if key in result:
            raise ValueError("duplicate key")
        result[key] = value
    return result


def canonical(raw):
    if len(raw) > 8192:
        raise ValueError("size")
    value = json.loads(raw.decode("utf-8"), object_pairs_hook=pairs)
    keys = ["amount", "domain", "memo", "operation", "tenant", "version"]
    if not isinstance(value, dict) or sorted(value) != keys:
        raise ValueError("keys")
    if any(not isinstance(v, str) for v in value.values()):
        raise ValueError("strings only")
    for v in value.values():
        v.encode("utf-8", "strict")  # rejects unpaired surrogates
    if (value["version"] != "1" or value["domain"] != "agent-platform/intent"
            or value["operation"] != "pay" or not 0 < len(value["tenant"]) <= 128
            or len(value["memo"]) > 1024):
        raise ValueError("schema")
    amount = value["amount"]
    if not amount.isascii() or not amount.isdecimal() or not 0 < int(amount) < 2**256 or str(int(amount)) != amount:
        raise ValueError("uint256")
    out = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return {"canonical": out, "digest": hashlib.sha256(b"agent-platform:wire:v1\0" + out.encode()).hexdigest()}


if __name__ == "__main__":
    try:
        result = canonical(sys.stdin.buffer.read(8193))
    except (ValueError, UnicodeError, TypeError, RecursionError):
        sys.exit(2)
    print(json.dumps(result, ensure_ascii=False))
