import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createRequire} from 'node:module';
import solc from 'solc';
import pg from 'pg';
import {createPublicClient,createWalletClient,http as transport,keccak256,padHex,stringToHex} from 'viem';
import {privateKeyToAccount} from 'viem/accounts';
import {foundry} from 'viem/chains';
import {OpsClient,fixtureKey,fixtureKey2} from '../../stablecoin-ops/client/dist/client.js';

const require=createRequire(import.meta.url);
const artifact=(pkg,name)=>require(`${pkg}/build/${name}.json`);
const factoryABI=artifact('@uniswap/v2-core','UniswapV2Factory');
const routerABI=artifact('@uniswap/v2-periphery','UniswapV2Router02');
const wethABI=artifact('@uniswap/v2-periphery','WETH9');
const pc=createPublicClient({chain:foundry,transport:transport(process.env.ANVIL_URL,{retryCount:0}),cacheTime:0});
const owner=privateKeyToAccount('0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80');
const traderKey='0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d';
const trader=privateKeyToAccount(traderKey),payer=privateKeyToAccount(fixtureKey);
const wallet=account=>createWalletClient({account,chain:foundry,transport:transport(process.env.ANVIL_URL,{retryCount:0})});
const ow=wallet(owner),tw=wallet(trader),pw=wallet(payer);
const db=new pg.Client({connectionString:process.env.DATABASE_URL}),chainDB=new pg.Client({connectionString:process.env.CHAIN_DATABASE_URL});
await db.connect();await chainDB.connect();
const output=process.env.LAB_OUTPUT, env={...process.env};
const children=new Map(), checks=[];
let proxy,miner,mining=true,miningPromise,dropSend=false,lostSend=false,logFault='';
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function until(read,accept,label,seconds=35){
 const end=Date.now()+seconds*1000;let last;
 while(Date.now()<end){try{last=await read();if(accept(last))return last;}catch(e){last=String(e);}await pause(120);}
 throw new Error(`Timed out ${label}: ${JSON.stringify(last)}`);
}
const rpc=(method,params=[])=>pc.request({method,params});
const mined=hash=>pc.waitForTransactionReceipt({hash,pollingInterval:50,timeout:20000});
async function write(w,address,abi,functionName,args){const r=await mined(await w.writeContract({address,abi,functionName,args}));assert.equal(r.status,'success');return r;}
async function deploy(a,args=[]){const r=await mined(await ow.deployContract({abi:a.abi,bytecode:a.bytecode.startsWith('0x')?a.bytecode:'0x'+a.bytecode,args}));assert.equal(r.status,'success');return r.contractAddress;}
const read=(address,abi,functionName,args=[])=>pc.readContract({address,abi,functionName,args});
const query=async(sql,args=[])=>db.query(sql,args);
const one=async(sql,args=[])=>{const r=await query(sql,args);return r.rows[0];};
async function request(base,route,body,expected=200){const r=await fetch(base+route,{method:body===undefined?'GET':'POST',...(body===undefined?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}),signal:AbortSignal.timeout(25000)});const v=await r.json();assert.equal(r.status,expected,JSON.stringify(v));return v;}
const gateway=(route,body,expected)=>request(env.CHAIN_GATEWAY_URL,route,body,expected);
function start(name,extra={}){
 const fd=fs.openSync(path.join(output,name+'.log'),'a',0o600);
 const child=spawn(env[name.toUpperCase()+'_BIN'],[],{env:{...env,...extra},stdio:['ignore',fd,fd]});fs.closeSync(fd);children.set(name,child);return child;
}
async function stop(name){const child=children.get(name);if(child && child.exitCode===null && child.signalCode===null){const done=once(child,'exit');child.kill('SIGKILL');await done;}children.delete(name);}
async function startGateway(){start('gateway',{DATABASE_URL:env.CHAIN_DATABASE_URL,LISTEN_ADDR:new URL(env.CHAIN_GATEWAY_URL).host});await until(()=>gateway('/health'),v=>v.chain===31337,'gateway ready');}
async function pauseMiner(){mining=false;if(miningPromise)await miningPromise;}
async function checkActivity(mode,p,extra={}) {
 const child=spawn(env.LAB_CHAIN_TEST_BIN,['-test.run=^TestChainReviewIntegration$','-test.v'],{env:{...env,LAB_CHAIN_REVIEW_CASE:mode,LAB_PAYMENT_ID:p.id,...extra},stdio:'inherit'});
 const [code]=await once(child,'exit');assert.equal(code,0,'Activity regression '+mode);
}
async function fork(snapshot) {
 await pauseMiner();assert.equal(await rpc('evm_revert',[snapshot]),true);await rpc('anvil_dropAllTransactions');await rpc('evm_increaseTime',[11]);await rpc('anvil_mine',['0x3']);
}
const mark=(name,detail)=>{checks.push({name,result:'passed',detail});console.log('PASS',name);};
try{
 await until(()=>pc.getChainId(),v=>v===31337,'local Anvil');
 const sources={};for(const name of ['Token.sol','PaymentReceiver.sol'])sources[name]={content:fs.readFileSync(new URL(name,import.meta.url),'utf8')};
 const compiled=JSON.parse(solc.compile(JSON.stringify({language:'Solidity',sources,settings:{optimizer:{enabled:true,runs:200},evmVersion:'paris',outputSelection:{'*':{'*':['abi','evm.bytecode.object']}}}})));
 assert.equal((compiled.errors??[]).filter(e=>e.severity==='error').length,0,JSON.stringify(compiled.errors));
 const token=compiled.contracts['Token.sol'].Token,receiverBuild=compiled.contracts['PaymentReceiver.sol'].PaymentReceiver;
 const tokenBuild={abi:token.abi,bytecode:token.evm.bytecode.object};
 const tokenA=await deploy(tokenBuild,['LAB-USD',6,10n**16n]),tokenB=await deploy(tokenBuild,['LAB-DAI',6,10n**16n]);
 const weth=await deploy(wethABI),factory=await deploy(factoryABI,[owner.address]),router=await deploy(routerABI,[factory,weth]);
 const deadline=async()=>BigInt((await pc.getBlock()).timestamp)+300n;
 for(const address of [tokenA,tokenB])await write(ow,address,token.abi,'approve',[router,10n**15n]);
 await write(ow,router,routerABI.abi,'addLiquidity',[tokenA,tokenB,10n**12n,10n**12n,10n**12n,10n**12n,owner.address,await deadline()]);
 const pair=await read(factory,factoryABI.abi,'getPair',[tokenA,tokenB]);
 const receiver=await deploy({abi:receiverBuild.abi,bytecode:receiverBuild.evm.bytecode.object},[tokenA,trader.address]);
 await write(ow,tokenA,token.abi,'transfer',[payer.address,10n**11n]);
 await mined(await ow.sendTransaction({to:payer.address,value:10n**19n}));
 await write(pw,tokenA,token.abi,'approve',[receiver,10n**11n]);
 await write(tw,tokenA,token.abi,'approve',[router,10n**11n]);
 const config={router,token_in:tokenA,token_out:tokenB,pair,receiver,router_code_hash:keccak256(await pc.getCode({address:router})),receiver_code_hash:keccak256(await pc.getCode({address:receiver}))};
 fs.writeFileSync(path.join(output,'chain.json'),JSON.stringify(config));
 env.LAB_CONFIG=path.join(output,'chain.json');env.LAB_SIGNER_KEY=traderKey.slice(2);
 env.CHAIN_TOKEN_IN=tokenA;env.CHAIN_TOKEN_OUT=tokenB;env.PAYMENT_ASSET=`31337:${tokenA}:LABUSD:6`;
 env.LAB_RECEIVER=receiver;
 proxy=http.createServer(async(req,res)=>{try{
  let raw='';for await(const b of req)raw+=b;const q=JSON.parse(raw);
  const response=await fetch(env.ANVIL_URL,{method:'POST',headers:{'Content-Type':'application/json'},body:raw,signal:AbortSignal.timeout(15000)});const value=await response.json();
  if(q.method==='eth_sendRawTransaction' && dropSend){dropSend=false;lostSend=true;res.writeHead(503,{'Content-Type':'application/json'});res.end(JSON.stringify({error:'accepted transaction; lost response'}));return;}
  if(q.method==='eth_getLogs' && Array.isArray(value.result) && logFault){
   const signature=logFault==='paid'?'Paid(bytes32,address,uint256)':'Swap(address,uint256,uint256,uint256,uint256,address)';
   const i=value.result.findIndex(l=>l.topics[0]===keccak256(stringToHex(signature)));if(i>=0)value.result.splice(i,1);
  }
  res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify(value));
 }catch(e){res.writeHead(502,{'Content-Type':'application/json'});res.end(JSON.stringify({error:String(e)}));}});
 await new Promise(resolve=>proxy.listen(0,'127.0.0.1',resolve));env.RPC_URL=`http://127.0.0.1:${proxy.address().port}`;
 await startGateway();
 start('provider',{LISTEN_ADDR:new URL(env.CHANNEL_URL).host});start('api',{LISTEN_ADDR:new URL(env.API_URL).host});start('worker');
 await until(()=>request(env.API_URL,'/health'),v=>v.ok,'payment API ready');
 miner=setInterval(()=>{if(mining && !miningPromise){miningPromise=rpc('anvil_mine',['0x1']).finally(()=>{miningPromise=undefined;});}},350);
 const client=new OpsClient(env.API_URL,fixtureKey,env.PAYMENT_ASSET),other=new OpsClient(env.API_URL,fixtureKey2,env.PAYMENT_ASSET);
 await client.login();await other.login();
 const paymentState=(id,state,scope=client)=>until(()=>scope.get(id),v=>v.state===state,'payment '+state);
 const conversion=id=>client.call('/payments/'+id+'/conversion');
 const converted=(id,state='settled')=>until(()=>conversion(id),v=>v.state===state,'conversion '+state,50);
 const pay=async p=>write(pw,receiver,receiverBuild.abi,'pay',[padHex('0x'+p.id,{size:32}),BigInt(p.amount_atomic)]);
 const makePayment=async(key,value='100')=>{const p=await client.create(key,value);await client.approve(p.id);await paymentState(p.id,'pending');await pay(p);await paymentState(p.id,'settled');return p;};
 const quote=async p=>{const q=await client.call('/payments/'+p.id+'/conversion-quote');return {min_out:q.min_out,deadline:q.deadline};};
 const convert=async(p,q)=>client.call('/payments/'+p.id+'/convert','POST',q??await quote(p));
 const balance=async id=>BigInt((await one('SELECT balance::text FROM lab.accounts WHERE id=$1',[id])).balance);

 if(env.LAB_RECOVERY==='1'){
  const {runRecoveryLab}=await import('./payment-recovery-tests.mjs');
  await runRecoveryLab({env,pc,rpc,read,token,tokenB,trader,query,one,balance,client,makePayment,convert,converted,paymentState,pay,quote,gateway,stop,start,startGateway,pauseMiner,resumeMiner:()=>{mining=true;},fork,checkActivity,mark,until,pause,output,chainDB,dropNextSend:()=>{dropSend=true;}});
  if(env.LAB_SERVE!=='1')fs.writeFileSync(path.join(output,'payment-e2e.json'),JSON.stringify({result:'passed',checks,scope:'Local operator plans, actual same-nonce cancellation, separate payment/chain database commits, durable Temporal audit restart, synthetic assets only'},null,2)+'\n');
 }else if(env.LAB_SERVE==='1'){
  const keys=['API_URL','ANVIL_URL','PAYMENT_ASSET','LAB_RECEIVER','CHAIN_TOKEN_IN','CHAIN_GATEWAY_URL'];
  fs.writeFileSync(path.join(output,'lab.env'),keys.map(k=>`export ${k}='${env[k]}'`).join('\n')+'\n',{mode:0o600});
  console.log(`Fresh local session: source ${path.join(output,'lab.env')}\nnode examples/onchain-execution/client/payment-cli.mjs demo 10\nPress Enter or Ctrl-C to stop.`);
  process.stdin.resume();await once(process.stdin,'data');process.stdin.pause();
 }else{
 const created=await Promise.all(Array.from({length:8},()=>client.create('e2e-order','100')));assert.equal(new Set(created.map(p=>p.id)).size,1);const first=created[0];
 assert.equal((await other.request('/payments/'+first.id+'/conversion-quote')).status,404);
 assert.equal((await client.request('/payments/'+first.id+'/convert','POST',{min_out:'1',deadline:Number(await deadline())})).status,409);
 await query("INSERT INTO ops.faults VALUES('submit_response_lost',1),('settlement_response_lost',1),('notify_response_lost',1)");
 await client.approve(first.id);await paymentState(first.id,'pending');
 await stop('worker');logFault='paid';await pay(first);await gateway('/index/sync',{},422);
 assert.equal((await one("SELECT count(*) FROM lab.journals WHERE id=$1",['settle:'+first.id])).count,'0');
 start('worker');await pause(1800);assert.notEqual((await client.get(first.id)).state,'settled');logFault='';
 await paymentState(first.id,'settled');
 await until(()=>one('SELECT count(*) FROM ops.merchant_inbox WHERE event_id=$1',['settle:'+first.id]),v=>v.count==='1','settlement Inbox');
 assert.equal(await balance('merchant_a'),100000000n);
 const receivedEvidence=await one("SELECT payload->'evidence' AS proof FROM ops.events WHERE payment=$1 AND payload->>'state'='confirmed'",[first.id]);
 assert.equal(receivedEvidence.proof.reference,first.id);assert(receivedEvidence.proof.transaction.startsWith('0x'));

 assert.equal((await client.request('/webhook','POST',{})).status,409);
 assert.equal((await client.request('/payments/'+first.id+'/refund','POST')).status,409);
 mark('wallet_payment_indexer_gate_worker_restart_ledger_inbox',{payment:first.id,asset:env.PAYMENT_ASSET,amount:first.amount_atomic});

 await query("INSERT INTO ops.faults VALUES('conversion_commit_response_lost',1)");
 const terms=await quote(first);dropSend=true;logFault='swap';
 await Promise.all(Array.from({length:5},()=>convert(first,terms)));
 assert.equal((await client.request('/payments/'+first.id+'/convert','POST',{...terms,min_out:'1'})).status,409);
 await until(async()=>lostSend,v=>v,'lost send response');await stop('worker');await stop('gateway');
 const intentRow=(await chainDB.query("SELECT nonce,(SELECT count(*) FROM attempts WHERE intent=intents.id) AS attempts FROM intents WHERE id=$1",['convert_'+first.id])).rows[0];
 assert.equal(intentRow.attempts,'1');assert.equal(await balance('held:'+first.id),100000000n);assert.equal(await balance('merchant_a:out'),0n);
 await pause(600);await startGateway();start('worker');await pause(1500);
 assert.equal(await balance('merchant_a:out'),0n);assert.equal(await balance('held:'+first.id),100000000n);logFault='';
 const c1=await converted(first.id);assert(c1.output_atomic && BigInt(c1.output_atomic)>=BigInt(terms.min_out));
 assert.equal(await balance('held:'+first.id),0n);assert.equal(await balance('merchant_a'),0n);
 assert.equal(await read(tokenB,token.abi,'balanceOf',[trader.address]),BigInt(c1.output_atomic));
 await until(()=>one("SELECT count(*) FROM lab.outbox WHERE event_id=ANY($1::text[]) AND NOT published",[['settle:'+first.id,'hold:'+first.id,'swap-in:'+first.id,'swap-out:'+first.id]]),v=>v.count==='0','conversion notifications');
 assert.equal((await one("SELECT count(*) FROM lab.journals WHERE id=ANY($1::text[])",[['hold:'+first.id,'swap-in:'+first.id,'swap-out:'+first.id]])).count,'3');
 assert.equal((await chainDB.query('SELECT count(*) FROM attempts WHERE intent=$1',['convert_'+first.id])).rows[0].count,'1');
 mark('payment_to_defi_response_loss_restart_indexer_delay_atomic_settlement',{intent:c1.intent,nonce:intentRow.nonce,output:c1.output_atomic,evidence:c1.evidence});

 const rollback=await makePayment('db-rollback','20');
 await query(`CREATE FUNCTION conversion_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.state='settled' THEN RAISE EXCEPTION 'injected settlement failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_conversion BEFORE UPDATE ON ops.conversions FOR EACH ROW EXECUTE FUNCTION conversion_failure()`);
 await convert(rollback);await converted(rollback.id,'review');
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=$1',['swap-out:'+rollback.id])).count,'0');assert.equal(await balance('held:'+rollback.id),20000000n);
 await query('DROP TRIGGER fail_conversion ON ops.conversions; DROP FUNCTION conversion_failure()');
 await client.call('/payments/'+rollback.id+'/conversion/resume','POST');const c2=await converted(rollback.id);
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=$1',['swap-out:'+rollback.id])).count,'1');
 mark('cross_asset_commit_rollback_and_same_intent_resume',{payment:rollback.id,intent:c2.intent,output:c2.output_atomic});

 const reverted=await makePayment('execution-revert','10');const tight=await quote(reverted);
 await pauseMiner();await rpc('evm_setAutomine',[false]);await convert(reverted,{...tight,min_out:tight.min_out});
 const signed=await until(()=>gateway('/intents/convert_'+reverted.id),v=>v.state==='broadcast','pending conversion');
 // A separately signed, much higher fee transaction changes pool reserves first.
 const gasPrice=await pc.getGasPrice();
 const marketHash=await ow.writeContract({address:router,abi:routerABI.abi,functionName:'swapExactTokensForTokens',args:[2n*10n**11n,1n,[tokenA,tokenB],owner.address,await deadline()],gas:400000n,gasPrice:gasPrice*100n});
 await rpc('anvil_mine',['0x1']);await rpc('evm_setAutomine',[true]);mining=true;await mined(marketHash);
 const c3=await converted(reverted.id,'released');
 assert.equal((await gateway('/intents/'+c3.intent)).state,'reverted');
 assert.equal(await balance('held:'+reverted.id),0n);assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=$1',['swap-out:'+reverted.id])).count,'0');
 mark('canonical_revert_releases_hold_without_refund_or_output_credit',{payment:reverted.id,hash:signed.hashes[0]});

 // Wrong amount and unknown reference are observable custody receipts, not
 // permission to credit an unrelated invoice or merchant.
 const bad=await client.create('wrong-deposit','1');await client.approve(bad.id);await paymentState(bad.id,'pending');
 await write(pw,receiver,receiverBuild.abi,'pay',[padHex('0x'+bad.id,{size:32}),1n]);
 await pause(1800);assert.notEqual((await client.get(bad.id)).state,'settled');assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=$1',['settle:'+bad.id])).count,'0');
 mark('mismatched_deposit_and_tenant_cannot_credit',{unallocated_atomic:'1'});

 const held=BigInt((await one("SELECT COALESCE(sum(balance),0)::text AS total FROM lab.accounts WHERE id LIKE 'held:%'")).total);
 const custodyIn=await read(tokenA,token.abi,'balanceOf',[trader.address]),custodyOut=await read(tokenB,token.abi,'balanceOf',[trader.address]);
 assert.equal(custodyIn,await balance('merchant_a')+held+1n);assert.equal(custodyOut,await balance('merchant_a:out'));
 assert.equal((await one('SELECT count(*) FROM lab.balance_drift')).count,'0');
 assert.equal((await one('SELECT count(*) FROM (SELECT journal_id,currency FROM lab.entries GROUP BY journal_id,currency HAVING sum(delta)<>0) q')).count,'0');
 mark('custody_ledger_reconciliation_and_per_asset_balancing',{custody_in:custodyIn.toString(),custody_out:custodyOut.toString(),held:held.toString(),unallocated:'1'});

 // Prepare all real signed spends before any destructive fork. The gateway
 // must refuse new nonce allocation after the first unresolved reorg.
 const spent=await makePayment('consumed-output','4');
 const b=await other.create('stale-execution','2');await other.approve(b.id);await paymentState(b.id,'pending',other);await pay(b);await paymentState(b.id,'settled',other);
 const missing=await other.create('missing-intent','1');await other.approve(missing.id);await paymentState(missing.id,'pending',other);
 await pauseMiner();const missingSnapshot=await rpc('evm_snapshot');mining=true;await pay(missing);await paymentState(missing.id,'settled',other);
 await pauseMiner();const spentSnapshot=await rpc('evm_snapshot');mining=true;
 await convert(spent);const spentConversion=await converted(spent.id);
 const missingTerms=await other.call('/payments/'+missing.id+'/conversion-quote');
 await checkActivity('create-hold',missing,{LAB_MERCHANT:'merchant_b',LAB_CONVERSION_REQUEST:JSON.stringify({min_out:missingTerms.min_out,deadline:missingTerms.deadline})});
 const bTerms=await other.call('/payments/'+b.id+'/conversion-quote');
 await checkActivity('create-hold',b,{LAB_MERCHANT:'merchant_b',LAB_CONVERSION_REQUEST:JSON.stringify({min_out:bTerms.min_out,deadline:bTerms.deadline})});
 await gateway('/intents/convert_'+b.id,{amount:b.amount_atomic,min_out:bTerms.min_out,deadline:bTerms.deadline});await gateway('/intents/convert_'+b.id+'/broadcast',{});await rpc('anvil_mine',['0x2']);await gateway('/index/sync',{});
 const oldSuccess=await gateway('/evidence/executions/convert_'+b.id);assert.equal(oldSuccess.state,'confirmed');

 const beforeIn=await balance('merchant_a'),beforeOut=await balance('merchant_a:out');
 const preReorgCustodyIn=await read(tokenA,token.abi,'balanceOf',[trader.address]),preReorgCustodyOut=await read(tokenB,token.abi,'balanceOf',[trader.address]);
 await pauseMiner();const snapshot=await rpc('evm_snapshot');mining=true;
 const reorg=await makePayment('full-reorg','5');await convert(reorg);const beforeReorg=await converted(reorg.id);
 await until(()=>one("SELECT count(*) FROM lab.outbox WHERE event_id=$1 AND published",['swap-out:'+reorg.id]),v=>v.count==='1','pre-reorg notification');
 await stop('worker');await pauseMiner();
 await query("INSERT INTO ops.faults(name,remaining) VALUES('notify_response_lost',1) ON CONFLICT(name) DO UPDATE SET remaining=1");
 assert.equal(await rpc('evm_revert',[snapshot]),true);await rpc('anvil_dropAllTransactions');await rpc('evm_increaseTime',[9]);await rpc('anvil_mine',['0x3']);mining=true;start('worker');
 await paymentState(reorg.id,'review_reversed');await converted(reorg.id,'review_reversed');
 assert.equal(await balance('merchant_a'),beforeIn);assert.equal(await balance('merchant_a:out'),beforeOut);
 assert.equal(await read(tokenA,token.abi,'balanceOf',[trader.address]),preReorgCustodyIn);assert.equal(await read(tokenB,token.abi,'balanceOf',[trader.address]),preReorgCustodyOut);
 const reversals=['undo-out:','undo-in:','release:','reorg:'].map(p=>p+reorg.id);
 await until(()=>one('SELECT count(*) FROM lab.outbox WHERE event_id=ANY($1::text[]) AND published',[reversals]),v=>v.count==='4','reorg notifications');
 assert.equal((await one('SELECT count(*) FROM ops.merchant_inbox WHERE event_id=ANY($1::text[])',[reversals])).count,'4');
 assert.equal((await chainDB.query('SELECT count(*) FROM attempts WHERE intent=$1',[beforeReorg.intent])).rows[0].count,'1');
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=ANY($1::text[])',[reversals])).count,'4');
 const attemptsBefore=(await chainDB.query('SELECT count(*) FROM attempts')).rows[0].count;
 await gateway('/intents/unsafe-after-reorg',{amount:'1000000',min_out:'1',deadline:Number(await deadline())},409);
 assert.equal((await chainDB.query('SELECT count(*) FROM attempts')).rows[0].count,attemptsBefore);
 mark('post_settlement_reorg_compensates_conversion_then_payment_once',{payment:reorg.id,reversals,original_evidence:beforeReorg.evidence});


 // A delivery outage must not terminate the financial monitor. Revert an
 // already credited payment after the initial Notify retries have exhausted.
 await pauseMiner();const notifySnapshot=await rpc('evm_snapshot');mining=true;
 await query("INSERT INTO ops.faults(name,remaining) VALUES('notify_response_lost',100) ON CONFLICT(name) DO UPDATE SET remaining=100");
 const notifyPayment=await makePayment('notification-audit-outage','3');await pause(4200);
 await fork(notifySnapshot);mining=true;
 await paymentState(notifyPayment.id,'review_reversed');
 await query("UPDATE ops.faults SET remaining=0 WHERE name='notify_response_lost'");
 await until(()=>one('SELECT count(*) FROM lab.outbox WHERE event_id=$1 AND published',['reorg:'+notifyPayment.id]),v=>v.count==='1','notification retry after financial recovery');
 mark('notification_outage_does_not_stop_reorg_monitor',{payment:notifyPayment.id});

 // Remove both prepared spends after all signing has finished.
 await stop('worker');
 await query("INSERT INTO lab.accounts VALUES('consumed_output','LABDAI',0,0)");
 const totalOut=await balance('merchant_a:out');
 await query("SELECT lab.transfer('fixture-consumed-output','merchant_a:out','consumed_output','LABDAI',$1)",[totalOut.toString()]);
 await fork(spentSnapshot);
 await checkActivity('audit',spent,{LAB_EXPECT_STATE:'review_chain'});
 const untouched=['undo-out:','undo-in:','release:'].map(x=>x+spent.id);
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=ANY($1::text[])',[untouched])).count,'0');
 assert.equal((await conversion(spent.id)).state,'settled');
 assert.match((await one("SELECT audit_evidence->>'compensation_error' AS error FROM ops.payments WHERE id=$1",[spent.id])).error,/insufficient funds/);
 assert.equal((await client.request('/payments/'+spent.id+'/reconcile','POST')).status,409);
 assert.equal((await client.request('/payments/'+spent.id+'/conversion/resume','POST')).status,409);
 assert.equal((await client.request('/payments/'+first.id+'/convert','POST',terms)).status,409);
 await checkActivity('submit-rejected',rollback);
 mark('consumed_output_atomic_rollback_and_merchant_quarantine',{payment:spent.id,output:spentConversion.output_atomic});

 // Restore only the fixture's internal transfer; this does not clear the
 // quarantine. Remaining cases construct existing Activities' inputs directly.
 await query("SELECT lab.reverse_transfer('fixture-restore-output','fixture-consumed-output')");
 // A provider has already observed confirmation, but the branch is replaced
 // with the same reference in a block that has only one confirmation.
 start('worker');
 const stale=await client.create('stale-deposit','2');await client.approve(stale.id);await paymentState(stale.id,'pending');await stop('worker');
 await pauseMiner();const depositSnapshot=await rpc('evm_snapshot');await pay(stale);await rpc('anvil_mine',['0x2']);
 await checkActivity('observe',stale);
 await fork(depositSnapshot);await pay(stale);
 await checkActivity('settle-rejected',stale);
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=$1',['settle:'+stale.id])).count,'0');
 mark('replaced_unconfirmed_deposit_cannot_reuse_confirmed_inbox',{payment:stale.id});

 // Both prepared spends were removed by spentSnapshot. Retry the old
 // success proof directly; no new authorization is created after quarantine.
 await checkActivity('stale-finish',b,{LAB_OLD_EVIDENCE:JSON.stringify({...oldSuccess,state:'reverted',amount:'',input:''})});
 await checkActivity('stale-finish',b,{LAB_OLD_EVIDENCE:JSON.stringify(oldSuccess)});
 assert.equal(await balance('held:'+b.id),2000000n);assert.equal(await balance('merchant_b:out'),0n);
 assert.equal((await one('SELECT count(*) FROM lab.journals WHERE id=$1',['swap-out:'+b.id])).count,'0');
 mark('orphaned_success_cannot_settle_from_stale_activity_argument',{payment:b.id});

 // A hold exists but Submit never reached the gateway. Losing the deposit must
 // still quarantine it; a 404 execution proof must not suppress the audit.
 await fork(missingSnapshot);
 await checkActivity('audit',missing,{LAB_EXPECT_STATE:'review_chain'});
 await checkActivity('submit-rejected',missing);
 assert.equal((await chainDB.query('SELECT count(*) FROM intents WHERE id=$1',['convert_'+missing.id])).rows[0].count,'0');
 assert.equal(await balance('held:'+missing.id),1000000n);
 assert.equal((await client.request('/payments/'+missing.id+'/conversion/resume','POST')).status,404);
 mark('missing_intent_does_not_hide_orphaned_deposit_or_release_hold',{payment:missing.id});
 assert.equal((await one('SELECT count(*) FROM lab.balance_drift')).count,'0');
 fs.writeFileSync(path.join(output,'payment-e2e.json'),JSON.stringify({result:'passed',checks,contracts:config,scope:'Actual local payment reference contract, synthetic stablecoins, Uniswap V2, Go payment/gateway/worker, separate PostgreSQL databases, Temporal and merchant Inbox; no public-chain funds or PSP'},null,2)+'\n');

 }
}finally{
 if(miner)clearInterval(miner);await pauseMiner();
 for(const name of [...children.keys()].reverse())await stop(name);
 if(proxy){proxy.closeAllConnections();await new Promise(resolve=>proxy.close(resolve));}
 await db.end();await chainDB.end();
}
