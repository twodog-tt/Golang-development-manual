import assert from 'node:assert/strict';
import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import { createHmac } from 'node:crypto';
import { openSync, closeSync, writeFileSync } from 'node:fs';
import { once } from 'node:events';
import { setTimeout as sleep } from 'node:timers/promises';
import { OpsClient, fixtureKey2, atomic, asset, type Payment } from './client.js';

const base = process.env.API_URL!;
const pg = process.env.LAB_PG_CONTAINER!;
const secret = process.env.LAB_CHANNEL_SECRET!;
const checks: string[] = [];
const workers: ChildProcess[] = [];
function sql(query: string): string {return execFileSync('docker',['exec',pg,'psql','-XAt','-v','ON_ERROR_STOP=1','-U','lab','-d','lab','-c',query],{encoding:'utf8'}).trim();}
function lit(value: string) {return "'" + value.replaceAll("'","''") + "'";}
function startWorker(legacy=false) {
  const fd = openSync(process.env.LAB_OUTPUT! + '/worker.log','a',0o600);
  const child = spawn(process.env.WORKER_BIN!,[],{env:{...process.env,LAB_LEGACY_WORKER:legacy?'1':''},stdio:['ignore',fd,fd]});closeSync(fd);workers.push(child);return child;
}
async function waitFor<T>(read:()=>Promise<T>,accept:(value:T)=>boolean,label:string,seconds=40):Promise<T> {
  const deadline = Date.now()+seconds*1000;
  while(Date.now()<deadline) {const value=await read();if(accept(value))return value;await sleep(150);}
  throw new Error('Timed out: '+label);
}
const client = new OpsClient(base);
const other = new OpsClient(base,fixtureKey2);
async function state(id:string,want:string,seconds=40) {return waitFor(()=>client.get(id),p=>p.state===want,want,seconds);}
function mark(name:string) {checks.push(name);console.log('PASS '+name);}
async function callback(event:Record<string,unknown>,options:{bad?:boolean;old?:boolean}={}) {
  const raw=JSON.stringify(event);const timestamp=String(Math.floor(Date.now()/1000)-(options.old?120:0));
  const signature=createHmac('sha256',secret).update(timestamp+'.'+raw).digest('hex');
  return fetch(base+'/webhook',{method:'POST',headers:{'X-Timestamp':timestamp,'X-Signature':options.bad?'00':signature},body:raw});
}
const event=(p:Payment,id:string,sequence:number,status='confirmed')=>({id,payment:p.id,amount_atomic:p.amount_atomic,asset,state:status,sequence});
let firstPayment='';
try {
  let worker=startWorker(true);
  await client.login();await other.login();mark('viem-to-Go SIWE message bytes and EOA verification');
  const challenge=await client.challenge();
  const bad=await client.account.signMessage({message:challenge.message.replace('URI:','URI: https://evil.example/')});
  assert.equal((await client.request('/login','POST',{id:challenge.id,signature:bad})).status,401);
  const signature=await client.account.signMessage({message:challenge.message});
  const concurrent=await Promise.all([client.request('/login','POST',{id:challenge.id,signature}),client.request('/login','POST',{id:challenge.id,signature})]);
  assert.deepEqual(concurrent.map(r=>r.status).sort(),[200,401]);
  const expired=await client.challenge();sql(`UPDATE ops.challenges SET expires=now()-interval '1 second' WHERE id=${lit(expired.id)}`);
  const expiredSig=await client.account.signMessage({message:expired.message});
  assert.equal((await client.request('/login','POST',{id:expired.id,signature:expiredSig})).status,401);
  mark('altered scope, expired nonce and concurrent nonce replay rejected');
  assert.equal(atomic('9007199254.740993'),'9007199254740993');
  assert.throws(()=>atomic('0.0000001'));assert.throws(()=>atomic('1e6'));
  const big=await client.create('big-integer','9007199254.740993');assert.equal(big.amount_atomic,'9007199254740993');
  const created=await Promise.all(Array.from({length:8},()=>client.create('same-payment','1.000001')));
  assert.equal(new Set(created.map(p=>p.id)).size,1);const payment=created[0];firstPayment=payment.id;
  assert.equal((await client.request('/payments','POST',{amount_atomic:'2',asset},{'Idempotency-Key':'same-payment'})).status,409);
  assert.equal((await other.request('/payments/'+payment.id,'GET',undefined,{'X-Merchant-Internal':'merchant_a'})).status,404);
  const otherPayment=await other.create('same-payment','1');assert.notEqual(otherPayment.id,payment.id);
  await other.call('/session','DELETE');assert.equal((await other.request('/ops')).status,401);
  mark('integer precision, concurrent idempotency, intent conflict, tenant isolation and logout');
  sql("INSERT INTO ops.faults VALUES('submit_response_lost',1),('settlement_response_lost',1),('refund_response_lost',1),('notify_response_lost',1)");
  await client.approve(payment.id);await state(payment.id,'pending');
  assert.equal(sql(`SELECT count(*) FROM ops.channel WHERE id=${lit(payment.id)}`),'1');
  const died=once(worker,'exit');worker.kill('SIGKILL');await died;
  sql(`UPDATE ops.channel SET state='confirmed',sequence=1 WHERE id=${lit(payment.id)}`);
  const confirmed=event(payment,'event-confirmed',1);
  assert.equal((await callback(confirmed,{bad:true})).status,401);
  assert.equal((await callback(confirmed,{old:true})).status,401);
  assert.equal((await callback({...confirmed,amount_atomic:'2'})).status,409);
  assert.equal((await callback(confirmed)).status,200);assert.equal((await callback(confirmed)).status,200);
  assert.equal((await callback({...confirmed,state:'pending'})).status,409);
  assert.equal((await callback(event(payment,'event-regression',2,'pending'))).status,409);
  assert.equal((await callback({...confirmed,id:''})).status,409);
  assert.equal((await callback(event(payment,'event-old',0,'pending'))).status,200);
  assert.equal(sql(`SELECT count(*) FROM lab.journals WHERE id=${lit('settle:'+payment.id)}`),'0');
  worker=startWorker();await state(payment.id,'settled');
  assert.equal(sql(`SELECT count(*) FROM lab.journals WHERE id=${lit('settle:'+payment.id)}`),'1');
  assert.equal(sql(`SELECT balance FROM lab.accounts WHERE id='merchant_a'`),'1000001');
  await waitFor(async()=>sql(`SELECT published FROM lab.outbox WHERE event_id=${lit('settle:'+payment.id)}`),v=>v==='t','merchant notification');
  assert.equal(sql(`SELECT count(*) FROM ops.merchant_inbox WHERE event_id=${lit('settle:'+payment.id)}`),'1');
  await client.call('/payments/'+payment.id+'/notify','POST');
  mark('response loss, signed webhook failures, duplicate/out-of-order events, real legacy worker SIGKILL and upgrade restart, exactly-one journal and merchant Inbox');
  await client.refund(payment.id);await state(payment.id,'refunded');
  await client.refund(payment.id);assert.equal(sql(`SELECT count(*) FROM lab.journals WHERE id=${lit('refund:'+payment.id)}`),'1');
  assert.equal(sql("SELECT balance FROM lab.accounts WHERE id='merchant_a'"),'0');
  mark('refund response loss, retry recovery and single accounting reversal');
  const timeout=await client.create('approval-timeout','1');await state(timeout.id,'expired',15);
  assert.equal(sql(`SELECT count(*) FROM ops.channel WHERE id=${lit(timeout.id)}`),'0');
  mark('durable approval timer expires without external transfer');
  const unknown=await client.create('unknown-outcome','2');await client.approve(unknown.id);await state(unknown.id,'review',30);
  assert.equal((await client.request('/payments/'+unknown.id+'/refund','POST')).status,409);
  assert.equal(sql(`SELECT count(*) FROM lab.journals WHERE id=${lit('settle:'+unknown.id)}`),'0');
  sql(`UPDATE ops.channel SET state='confirmed',sequence=1 WHERE id=${lit(unknown.id)}`);
  await client.call('/payments/'+unknown.id+'/reconcile','POST');await state(unknown.id,'settled');
  await client.refund(unknown.id);await state(unknown.id,'refunded');
  mark('unknown outcome enters review, cannot refund, query-only reconciliation recovers it');
  const reorg=await client.create('reversal','3');await client.approve(reorg.id);await state(reorg.id,'pending');
  sql(`UPDATE ops.channel SET state='confirmed',sequence=1 WHERE id=${lit(reorg.id)}`);await state(reorg.id,'settled');
  assert.equal((await callback(event(reorg,'event-reversed',2,'reversed'))).status,200);
  await state(reorg.id,'review_reversed');
  assert.equal((await callback(event(reorg,'event-stale-confirmed',1))).status,200);
  assert.equal((await client.request('/payments/'+reorg.id+'/refund','POST')).status,409);
  assert.equal(sql(`SELECT count(*) FROM lab.journals WHERE id=${lit('reorg:'+reorg.id)}`),'1');
  mark('post-settlement provider reversal, stale confirmation and refund protection');
  const quoteResponse=await fetch(process.env.CHANNEL_URL!+'/transfers',{method:'POST',headers:{'Content-Type':'application/json','X-Lab-Token':secret},body:JSON.stringify({...payment,id:'f'.repeat(48),quote_expires:'2000-01-01T00:00:00Z'})});
  assert.equal(quoteResponse.status,409);assert.equal(sql("SELECT count(*) FROM ops.channel WHERE id='"+'f'.repeat(48)+"'"),'0');
  const clean=await client.call<{drift:boolean}>('/ops');assert.equal(clean.drift,false);
  sql("UPDATE lab.accounts SET balance=balance+1 WHERE id='merchant_a'");
  assert.equal((await client.call<{drift:boolean}>('/ops')).drift,true);
  sql("UPDATE lab.accounts SET balance=balance-1 WHERE id='merchant_a'");
  assert.equal((await client.call<{drift:boolean}>('/ops')).drift,false);
  assert.equal(sql("SELECT count(*) FROM lab.balance_drift"),'0');
  mark('expired quote rejected and admin-injected balance drift detected');
  execFileSync(process.env.HISTORY_BIN!,['-id','pay:'+firstPayment,'-out',process.env.LAB_OUTPUT!+'/history.json'],{env:process.env,stdio:'inherit'});
  execFileSync(process.env.HISTORY_BIN!,['-id','pay:'+firstPayment,'-out',process.env.LAB_OUTPUT!+'/history-negative.json','-negative'],{env:process.env,stdio:'inherit'});
  mark('real Temporal history replay passes and incompatible workflow is rejected');
  await client.call('/wallets/'+client.account.address,'DELETE');
  assert.equal((await client.request('/ops')).status,401);
  assert.equal((await client.request('/challenge','POST',{address:client.account.address})).status,401);
  assert.equal(sql(`SELECT count(*) FROM lab.journals WHERE id=${lit('settle:'+payment.id)}`),'1');
  mark('wallet unbinding revokes sessions and challenges without deleting ledger facts');
  writeFileSync(process.env.LAB_OUTPUT!+'/integration.json',JSON.stringify({result:'passed',checks},null,2)+'\n');
} finally {
  for(const child of workers){if(child.exitCode===null && child.signalCode===null){const done=once(child,'exit');child.kill('SIGINT');await Promise.race([done,sleep(5000)]);if(child.exitCode===null && child.signalCode===null)child.kill('SIGKILL');}}
}
