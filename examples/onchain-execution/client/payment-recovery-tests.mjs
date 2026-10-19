import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {openRecovery} from './recovery.mjs';
import {privateKeyToAccount} from 'viem/accounts';

export async function runRecoveryLab(h){
 const {env,pc,rpc,read,token,tokenB,trader,query,one,balance,client,makePayment,convert,converted,paymentState,pay,quote,gateway,stop,start,startGateway,pauseMiner,resumeMiner,fork,checkActivity,mark,until,pause,output}=h;
 const withRecovery=async(fn,extra={})=>{const r=await openRecovery({...env,...extra});try{return await fn(r);}finally{await r.close();}};
 const plan=(action,target,ttl)=>withRecovery(r=>r.plan(action,target,ttl));
 const apply=(p,extra={})=>withRecovery(r=>r.apply(p,'operator-alice','LAB-RECOVERY'),extra);
 async function cli(p,extra={},ok=true){
  const file=path.join(output,p.id+'.plan.json');fs.writeFileSync(file,JSON.stringify(p),{mode:0o600});
  const child=spawn(process.execPath,[new URL('recovery.mjs',import.meta.url).pathname,'apply',file,'--operator','operator-alice','--ticket','LAB-RECOVERY'],{env:{...env,...extra},stdio:['ignore','pipe','pipe']});
  let stdout='',stderr='';child.stdout.on('data',b=>stdout+=b);child.stderr.on('data',b=>stderr+=b);
  const [code]=await once(child,'exit');assert.equal(code,ok?0:1,stderr||stdout);return ok?JSON.parse(stdout):stderr;
 }
 async function restoreFixture(p){await query('SELECT lab.reverse_transfer($1,$2)',['fixture-return:'+p.id,'fixture-consume:'+p.id]);}
 async function isolated(key,amount='10'){
  const p=await makePayment(key,amount);await pauseMiner();const snapshot=await rpc('evm_snapshot');resumeMiner();
  await convert(p);const c=await converted(p.id);await stop('worker');
  await query("INSERT INTO lab.accounts VALUES('consumed_output','LABDAI',0,0) ON CONFLICT DO NOTHING");
  const out=await balance('merchant_a:out');await query("SELECT lab.transfer($1,'merchant_a:out','consumed_output','LABDAI',$2)",['fixture-consume:'+p.id,out.toString()]);
  await fork(snapshot);await checkActivity('audit',p,{LAB_EXPECT_STATE:'review_chain'});
  assert.equal((await gateway('/intents/'+c.intent)).review_required,true);
  return {p,c};
 }
 const first=await isolated('operator-rebind');await restoreFixture(first.p);
 if(env.LAB_SERVE==='1'){
  const keys=['API_URL','ANVIL_URL','RPC_URL','PAYMENT_ASSET','LAB_RECEIVER','CHAIN_TOKEN_IN','CHAIN_GATEWAY_URL','DATABASE_URL','CHAIN_DATABASE_URL','LAB_CONFIG','LAB_SIGNER_KEY'];
  fs.writeFileSync(path.join(output,'lab.env'),keys.map(k=>`export ${k}='${env[k]}'`).join('\n')+'\n',{mode:0o600});
  console.log(`Recovery case: payment=${first.p.id} intent=${first.c.intent}\nsource ${path.join(output,'lab.env')}\nThe old swap is orphaned, output fixture returned, worker is stopped. Inspect, cancel this intent, compensate this payment, then start the worker by pressing Enter. The runner retains the session until a second Enter.`);
  process.stdin.resume();await once(process.stdin,'data');start('worker');resumeMiner();
  console.log('Worker resumed; wait for recovery audit before planning release-signer. Press Enter again to stop.');
  await once(process.stdin,'data');process.stdin.pause();return;
 }
 await assert.rejects(()=>plan('release-signer','signer'),/review|phases/);
 await assert.rejects(()=>plan('rebind',first.p.id),/execution/);
 // Actual network reinclusion of the original bytes, not an operator rebroadcast.
 const raw=(await h.chainDB.query('SELECT raw FROM attempts WHERE intent=$1 ORDER BY seq LIMIT 1',[first.c.intent])).rows[0].raw;
 await rpc('eth_sendRawTransaction',['0x'+raw.toString('hex')]);await rpc('anvil_mine',['0x3']);
 const rebind=await plan('rebind',first.p.id);
 const tampered={...rebind,target:'another-payment'};await assert.rejects(()=>apply(tampered),/altered/);
 const expiring=await plan('rebind',first.p.id,1);await pause(1200);await assert.rejects(()=>apply(expiring),/expired/);
 const before=Number((await one('SELECT count(*) FROM lab.journals')).count);
 await cli(rebind,{LAB_RECOVERY_FAULT:'commit-response-lost'},false);
 assert.equal((await client.get(first.p.id)).state,'settled');await cli(rebind);await cli(rebind);
 assert.equal(Number((await one('SELECT count(*) FROM lab.journals')).count),before);
 assert.equal((await one('SELECT count(*) FROM ops.audit WHERE action=$1',['operator:'+rebind.id])).count,'1');
 await assert.rejects(()=>withRecovery(r=>r.apply(rebind,'operator-bob','LAB-RECOVERY')),/identity/);
 await assert.rejects(()=>plan('release-signer','signer'),/audit/);
 await query("INSERT INTO ops.faults VALUES('recovery_monitor_response_lost',1)");start('worker');
 await until(()=>one('SELECT monitor_started,monitor_checked FROM ops.recovery_plans WHERE id=$1',[rebind.id]),v=>v.monitor_started&&v.monitor_checked,'durable audit restart');
 await checkActivity('monitor-check',first.p,{LAB_RECOVERY_PLAN:rebind.id});
 await query("SELECT lab.transfer('fixture-liability-diversion','merchant_a:out','consumed_output','LABDAI',1)");
 await assert.rejects(()=>plan('release-signer','signer'),/liabilities/);
 await query("SELECT lab.reverse_transfer('fixture-liability-return','fixture-liability-diversion')");
 const pendingSnapshot=await rpc('evm_snapshot');await rpc('evm_setAutomine',[false]);
 const signer=privateKeyToAccount('0x'+env.LAB_SIGNER_KEY.replace(/^0x/,''));
 const pendingRaw=await signer.signTransaction({chainId:31337,type:'legacy',nonce:await pc.getTransactionCount({address:signer.address}),to:signer.address,value:0n,gas:21000n,gasPrice:await pc.getGasPrice()});
 await rpc('eth_sendRawTransaction',[pendingRaw]);await assert.rejects(()=>plan('release-signer','signer'),/Pending transactions/);
 assert.equal(await rpc('evm_revert',[pendingSnapshot]),true);await rpc('anvil_dropAllTransactions');await rpc('evm_setAutomine',[true]);
 const release1=await plan('release-signer','signer');await apply(release1);
 assert.equal((await gateway('/intents/'+first.c.intent)).review_required,false);
 mark('operator_rebind_immutable_expiry_idempotent_commit_loss_and_audit_restart',{payment:first.p.id,plan:rebind.id,release:release1.id});
 resumeMiner();
 const healthy=await makePayment('after-first-recovery','1');await convert(healthy);await converted(healthy.id);
 const second=await isolated('operator-cancel','4');
 const expiryCancel=await plan('cancel',second.c.intent,1);await pause(1200);await assert.rejects(()=>apply(expiryCancel),/expired/);
 // A new branch invalidates the approved facts even when nonce/balances match.
 const snap=await rpc('evm_snapshot');await rpc('anvil_mine',['0x2']);
 const stale=await plan('cancel',second.c.intent);await fork(snap);
 await assert.rejects(()=>apply(stale),/changed/);
 const cancellation=await plan('cancel',second.c.intent);
 await cli(cancellation,{LAB_RECOVERY_FAULT:'commit-response-lost'},false);
 assert.equal((await h.chainDB.query('SELECT count(*) FROM intent_cancellations WHERE intent=$1',[second.c.intent])).rows[0].count,'1');
 await stop('gateway');h.dropNextSend();
 await cli(cancellation,{ANVIL_URL:env.RPC_URL},false); // accepted by RPC, ACK lost
 await startGateway();
 await assert.rejects(()=>plan('compensate',second.p.id),/confirmed|cancellation/);
 await rpc('anvil_mine',['0x3']);await gateway('/index/sync',{});const sent=await cli(cancellation);
 assert.equal(sent.hash,(await h.chainDB.query('SELECT hash FROM intent_cancellations WHERE intent=$1',[second.c.intent])).rows[0].hash);
 assert.equal((await gateway('/evidence/executions/'+second.c.intent)).state,'cancelled');
 await assert.rejects(()=>plan('compensate',second.p.id),/Insufficient output/);
 mark('same_nonce_cancellation_durable_raw_unknown_send_restart_and_depth_gate',{payment:second.p.id,hash:sent.hash,plan:cancellation.id});
 await restoreFixture(second.p);
 const compensation=await plan('compensate',second.p.id);
 await query("CREATE FUNCTION fail_operator_apply() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.applied THEN RAISE EXCEPTION 'injected operator commit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_operator BEFORE UPDATE ON ops.recovery_plans FOR EACH ROW EXECUTE FUNCTION fail_operator_apply()");
 await assert.rejects(()=>apply(compensation),/injected/);
 const ids=['undo-out:','undo-in:','release:'].map(prefix=>prefix+second.p.id);
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=ANY($1::text[])',[ids])).count,'0');
 assert.equal((await client.get(second.p.id)).state,'review_chain');
 await query('DROP TRIGGER fail_operator ON ops.recovery_plans; DROP FUNCTION fail_operator_apply()');
 await Promise.all(Array.from({length:4},()=>apply(compensation)));
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=ANY($1::text[])',[ids])).count,'3');
 assert.equal((await converted(second.p.id,'recovered_cancelled')).state,'recovered_cancelled');
 start('worker');await until(()=>one('SELECT monitor_started,monitor_checked FROM ops.recovery_plans WHERE id=$1',[compensation.id]),v=>v.monitor_started&&v.monitor_checked,'compensation audit');
 await until(()=>one('SELECT count(*) FROM lab.outbox WHERE event_id=ANY($1::text[]) AND NOT published',[ids]),v=>v.count==='0','compensation notifications');
 const release2=await plan('release-signer','signer');await cli(release2,{LAB_RECOVERY_FAULT:'commit-response-lost'},false);await cli(release2);
 assert.equal((await gateway('/intents/'+second.c.intent)).review_required,false);
 mark('atomic_compensation_parallel_approval_outbox_and_safe_signer_release',{payment:second.p.id,plan:compensation.id,release:release2.id});
 resumeMiner();
 const finalHealthy=await makePayment('after-cancellation-recovery','1');await convert(finalHealthy);await converted(finalHealthy.id);
 mark('new_payment_conversion_after_both_databases_and_signer_recover',{payment:finalHealthy.id});

 const missing=await client.create('operator-no-intent','2');await client.approve(missing.id);await paymentState(missing.id,'pending');
 await pauseMiner();const depositSnap=await rpc('evm_snapshot');resumeMiner();await pay(missing);await paymentState(missing.id,'settled');await stop('worker');
 const terms=await quote(missing);await checkActivity('create-hold',missing,{LAB_MERCHANT:'merchant_a',LAB_CONVERSION_REQUEST:JSON.stringify(terms)});
 await fork(depositSnap);await checkActivity('audit',missing,{LAB_EXPECT_STATE:'review_chain'});
 const retire=await plan('compensate',missing.id);
 await query("CREATE TRIGGER fail_operator BEFORE UPDATE ON ops.recovery_plans FOR EACH ROW EXECUTE FUNCTION fail_operator_apply()").catch(async()=>{
  await query("CREATE FUNCTION fail_operator_apply() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.applied THEN RAISE EXCEPTION 'injected operator commit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_operator BEFORE UPDATE ON ops.recovery_plans FOR EACH ROW EXECUTE FUNCTION fail_operator_apply()");
 });
 await assert.rejects(()=>apply(retire),/injected/);
 assert.equal((await h.chainDB.query('SELECT plan FROM retired_intents WHERE id=$1',['convert_'+missing.id])).rows[0].plan,retire.id);
 assert.equal(await balance('held:'+missing.id),2000000n);
 await gateway('/intents/convert_'+missing.id,{amount:missing.amount_atomic,min_out:terms.min_out,deadline:terms.deadline},409);
 await query('DROP TRIGGER fail_operator ON ops.recovery_plans; DROP FUNCTION fail_operator_apply()');
 await cli(retire);await cli(retire);
 assert.equal((await client.get(missing.id)).state,'recovered_closed');assert.equal(await balance('held:'+missing.id),0n);
 start('worker');const closureIDs=['release:','reorg:'].map(x=>x+missing.id);
 await until(()=>one('SELECT count(*) FROM lab.outbox WHERE event_id=ANY($1::text[]) AND NOT published',[closureIDs]),v=>v.count==='0','closed-case notifications');
 assert.equal((await one('SELECT count(*) FROM ops.merchant_inbox WHERE event_id=ANY($1::text[])',[closureIDs])).count,'2');
 await h.chainDB.query('DELETE FROM retired_intents WHERE id=$1',['convert_'+missing.id]);
 await assert.rejects(()=>plan('release-signer','signer'),/fence/);
 await h.chainDB.query('INSERT INTO retired_intents(id,plan) VALUES($1,$2)',['convert_'+missing.id,retire.id]);
 const finalRelease=await plan('release-signer','signer');await apply(finalRelease);
 const inspection=await withRecovery(r=>r.inspect());
 assert.equal(inspection.payments.filter(p=>p.state==='review_chain').length,0);
 assert.equal((await one('SELECT count(*) FROM lab.balance_drift')).count,'0');
 mark('never_created_intent_fence_survives_other_database_failure_and_closes_once',{payment:missing.id,plan:retire.id});
}
