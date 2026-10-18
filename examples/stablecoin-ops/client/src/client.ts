import { privateKeyToAccount } from 'viem/accounts';
import { createSiweMessage, parseSiweMessage } from 'viem/siwe';
import { parseUnits, type Hex } from 'viem';

// Public local-fixture keys only. Never fund these addresses on any real network.
export const fixtureKey = ('0x' + '0'.repeat(63) + '1') as Hex;
export const fixtureKey2 = ('0x' + '0'.repeat(63) + '2') as Hex;
export const asset = '31337:0x1111111111111111111111111111111111111111:USDC:6';
export function atomic(value: string): string {
  if (!/^(0|[1-9]\d*)(\.\d{1,6})?$/.test(value)) throw new Error('USDC requires <=6 decimal places; no rounding');
  const n = parseUnits(value, 6);
  if (n <= 0n || n > 10n ** 18n) throw new Error('amount outside lab bounds');
  return n.toString();
}
export type Payment = { id: string; amount_atomic: string; asset: string; state: string; merchant: string };
export class OpsClient {
  token = '';
  account;
  constructor(readonly base: string, key: Hex = fixtureKey) { this.account = privateKeyToAccount(key); }
  async request(path: string, method = 'GET', body?: unknown, extra: Record<string,string> = {}) {
    return fetch(this.base + path, {method, headers: {'Content-Type':'application/json', ...(this.token ? {Authorization:'Bearer ' + this.token} : {}), ...extra}, body: body === undefined ? undefined : JSON.stringify(body)});
  }
  async call<T = Record<string,unknown>>(path: string, method = 'GET', body?: unknown, extra: Record<string,string> = {}): Promise<T> {
    const response = await this.request(path,method,body,extra);
    if (!response.ok) throw new Error(`${method} ${path}: HTTP ${response.status}`);
    return response.json() as Promise<T>;
  }
  async challenge() {
    return this.call<{id:string;message:string}>('/challenge','POST',{address:this.account.address});
  }
  async login() {
    const challenge = await this.challenge();
    const parsed = parseSiweMessage(challenge.message);
    if (!parsed.address || !parsed.domain || !parsed.uri || !parsed.nonce || !parsed.issuedAt) throw new Error('Incomplete SIWE challenge');
    const expectedOrigin = new URL(this.base);
    if (parsed.domain !== expectedOrigin.host || parsed.uri !== this.base + '/ops' || parsed.chainId !== 31337 || parsed.address !== this.account.address) throw new Error('Challenge scope mismatch');
    const rebuilt = createSiweMessage({address:parsed.address,domain:parsed.domain,uri:parsed.uri,nonce:parsed.nonce,version:'1',chainId:31337,issuedAt:parsed.issuedAt,expirationTime:parsed.expirationTime,statement:parsed.statement});
    if (rebuilt !== challenge.message) throw new Error('Go/viem SIWE bytes differ');
    const signature = await this.account.signMessage({message:challenge.message});
    const result = await this.call<{token:string}>('/login','POST',{id:challenge.id,signature});
    this.token = result.token;
  }
  create(key: string, value: string) {return this.call<Payment>('/payments','POST',{amount_atomic:atomic(value),asset},{'Idempotency-Key':key});}
  get(id: string) {return this.call<Payment>('/payments/' + id);}
  approve(id: string) {return this.call('/payments/' + id + '/approve','POST');}
  refund(id: string) {return this.call('/payments/' + id + '/refund','POST');}
}
