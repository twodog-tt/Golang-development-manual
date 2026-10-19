// Node 22.15+ --experimental-strip-types. This is a fixed schema, not general JCS.
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';

function parseFlat(raw: string): Record<string, string> {
  let pos = 0;
  const value: Record<string, string> = Object.create(null);
  const whitespace = () => { while (/[\x20\t\r\n]/.test(raw[pos] ?? 'x')) pos++; };
  const expect = (char: string) => { whitespace(); if (raw[pos++] !== char) throw Error('syntax'); };
  const string = (): string => {
    whitespace();
    const pattern = /"(?:[^"\\\u0000-\u001f]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*"/y;
    pattern.lastIndex = pos;
    const match = pattern.exec(raw);
    if (!match) throw Error('string required');
    pos = pattern.lastIndex;
    const s: string = JSON.parse(match[0]);
    for (let i = 0; i < s.length; i++) {
      const n = s.charCodeAt(i);
      if (n >= 0xdc00 && n <= 0xdfff) throw Error('unpaired surrogate');
      if (n >= 0xd800 && n <= 0xdbff) {
        const low = s.charCodeAt(++i);
        if (!(low >= 0xdc00 && low <= 0xdfff)) throw Error('unpaired surrogate');
      }
    }
    return s;
  };
  expect('{'); whitespace();
  if (raw[pos] !== '}') {
    while (true) {
      const key = string(); expect(':');
      if (Object.hasOwn(value, key)) throw Error('duplicate key');
      value[key] = string(); whitespace();
      if (raw[pos] !== ',') break;
      pos++;
    }
  }
  expect('}'); whitespace(); if (pos !== raw.length) throw Error('trailing input');
  return value;
}

try {
  const raw = readFileSync(0);
  if (raw.length > 8192) throw Error('size');
  const value = parseFlat(new TextDecoder('utf-8', { fatal: true, ignoreBOM: true }).decode(raw));
  const keys = ['amount', 'domain', 'memo', 'operation', 'tenant', 'version'];
  if (JSON.stringify(Object.keys(value).sort()) !== JSON.stringify(keys)) throw Error('keys');
  if (value.version !== '1' || value.domain !== 'agent-platform/intent' || value.operation !== 'pay'
      || Array.from(value.tenant).length < 1 || Array.from(value.tenant).length > 128
      || Array.from(value.memo).length > 1024) throw Error('schema');
  if (!/^[1-9][0-9]*$/.test(value.amount) || BigInt(value.amount) >= 2n ** 256n) throw Error('uint256');
  const canonical = '{' + keys.map(k => JSON.stringify(k) + ':' + JSON.stringify(value[k])).join(',') + '}';
  const digest = createHash('sha256').update('agent-platform:wire:v1\0').update(canonical).digest('hex');
  console.log(JSON.stringify({ canonical, digest }));
} catch {
  process.exit(2);
}
