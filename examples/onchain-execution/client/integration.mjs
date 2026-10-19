import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createRequire} from 'node:module';
import solc from 'solc';
import pg from 'pg';
import {createPublicClient, createWalletClient, http as transport, keccak256, encodeFunctionData, parseAbi, decodeEventLog} from 'viem';
import {privateKeyToAccount} from 'viem/accounts';
import {foundry} from 'viem/chains';

const require=createRequire(import.meta.url);
const artifact=(pkg,name)=>require(`${pkg}/build/${name}.json`);
const factoryArtifact=artifact('@uniswap/v2-core','UniswapV2Factory');
const pairArtifact=artifact('@uniswap/v2-core','UniswapV2Pair');
const routerArtifact=artifact('@uniswap/v2-periphery','UniswapV2Router02');
const wethArtifact=artifact('@uniswap/v2-periphery','WETH9');
const chain=foundry;
const rpcURL=process.env.ANVIL_URL;
const publicClient=createPublicClient({chain,transport:transport(rpcURL,{retryCount:0})});
// Public Anvil development keys; never fund on a public network.
const owner=privateKeyToAccount('0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80');
const traderKey='0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d';
const trader=privateKeyToAccount(traderKey);
const wallet=account=>createWalletClient({account,chain,transport:transport(rpcURL,{retryCount:0})});
const ownerWallet=wallet(owner), traderWallet=wallet(trader);
const db=new pg.Client({connectionString:process.env.DATABASE_URL});await db.connect();
const output=process.env.LAB_OUTPUT;
const checks=[];
let child, proxy, dropSend=false, logFault='', wrongChain=false, trackHead=false, headRequests=0;
const pause=ms=>new Promise(r=>setTimeout(r,ms));
const rpc=(method,params=[])=>publicClient.request({method,params});
const read=(address,abi,functionName,args=[],extra={})=>publicClient.readContract({address,abi,functionName,args,...extra});
const mined=hash=>publicClient.waitForTransactionReceipt({hash,timeout:15000,pollingInterval:50});
async function write(w,address,abi,functionName,args){return mined(await w.writeContract({address,abi,functionName,args}));}
async function deploy(abi,bytecode,args=[]){
 const r=await mined(await ownerWallet.deployContract({abi,bytecode:bytecode.startsWith('0x')?bytecode:'0x'+bytecode,args}));
 assert.equal(r.status,'success');return r.contractAddress;
}
const record=(name,detail)=>{checks.push({name,result:'passed',detail});console.log('PASS',name);};
async function api(route,body,expected=200){
 const r=await fetch(process.env.API_URL+route,{method:body===undefined?'GET':'POST',...(body===undefined?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}),signal:AbortSignal.timeout(25000)});
 const value=await r.json();assert.equal(r.status,expected,`${route}: ${JSON.stringify(value)}`);return value;
}
async function stop(){if(child && child.exitCode===null && child.signalCode===null){const done=once(child,'exit');child.kill('SIGKILL');await done;}child=undefined;}
async function start(config){
 fs.writeFileSync(path.join(output,'config.json'),JSON.stringify(config));
 const log=fs.openSync(path.join(output,'gateway.log'),'a');
 child=spawn(process.env.SERVER_BIN,[],{env:{...process.env,LAB_CONFIG:path.join(output,'config.json'),LAB_SIGNER_KEY:traderKey.slice(2),RPC_URL:`http://127.0.0.1:${proxy.address().port}`,LISTEN_ADDR:new URL(process.env.API_URL).host},stdio:['ignore',log,log]});fs.closeSync(log);
 for(let i=0;i<100;i++){
  if(child.exitCode!==null)throw new Error(fs.readFileSync(path.join(output,'gateway.log'),'utf8'));
  try{await api('/health');return;}catch{await pause(50);}
 }throw new Error('gateway startup timed out');
}
try{
 assert.equal(await publicClient.getChainId(),31337);
 const input={language:'Solidity',sources:{'Token.sol':{content:fs.readFileSync(new URL('Token.sol',import.meta.url),'utf8')}},settings:{optimizer:{enabled:true,runs:200},evmVersion:'paris',outputSelection:{'*':{'*':['abi','evm.bytecode.object']}}}};
 const compiled=JSON.parse(solc.compile(JSON.stringify(input)));
 assert.equal((compiled.errors??[]).filter(x=>x.severity==='error').length,0,JSON.stringify(compiled.errors));
 const tokenArtifact=compiled.contracts['Token.sol'].Token;
 const tokenA=await deploy(tokenArtifact.abi,tokenArtifact.evm.bytecode.object,['LAB-USD',6,10n**16n]);
 const tokenB=await deploy(tokenArtifact.abi,tokenArtifact.evm.bytecode.object,['LAB-ETH',18,10n**28n]);
 const weth=await deploy(wethArtifact.abi,wethArtifact.bytecode);
 const factory=await deploy(factoryArtifact.abi,factoryArtifact.bytecode,[owner.address]);
 const router=await deploy(routerArtifact.abi,routerArtifact.bytecode,[factory,weth]);
 const tokenABI=tokenArtifact.abi, routerABI=routerArtifact.abi;
 const deadline=async()=>Number((await publicClient.getBlock()).timestamp)+300;
 for(const token of [tokenA,tokenB])await write(ownerWallet,token,tokenABI,'approve',[router,10n**27n]);
 await write(ownerWallet,router,routerABI,'addLiquidity',[tokenA,tokenB,10n**12n,10n**21n,10n**12n,10n**21n,owner.address,BigInt(await deadline())]);
 const pair=await read(factory,factoryArtifact.abi,'getPair',[tokenA,tokenB]);
 await write(ownerWallet,tokenA,tokenABI,'transfer',[trader.address,10n**11n]);
 await write(traderWallet,tokenA,tokenABI,'approve',[router,10n**11n]);
 const config={router,token_in:tokenA,token_out:tokenB,pair,router_code_hash:keccak256(await publicClient.getCode({address:router}))};
 // Fault proxy is only bound to loopback and forwards to this run's local Anvil.
 proxy=http.createServer(async(req,res)=>{
  try{
   let body='';for await(const chunk of req)body+=chunk;
   const request=JSON.parse(body);
   if(trackHead && request.method==='eth_getBlockByNumber' && request.params[0]==='latest')headRequests++;
   const upstream=await fetch(rpcURL,{method:'POST',headers:{'Content-Type':'application/json'},body,signal:AbortSignal.timeout(15000)});
   const value=await upstream.json();
   if(wrongChain && request.method==='eth_chainId')value.result='0x1';
   if(dropSend && request.method==='eth_sendRawTransaction'){
    dropSend=false;res.writeHead(503,{'Content-Type':'application/json'});res.end(JSON.stringify({error:'response lost after forwarding'}));return;
   }
   if(request.method==='eth_getLogs' && Array.isArray(value.result)){
    const idx=value.result.findIndex(l=>l.topics?.[0]===keccak256('0x5472616e7366657228616464726573732c616464726573732c75696e7432353629'));
    if(idx>=0 && logFault==='omit')value.result.splice(idx,1);
    if(idx>=0 && logFault==='poison')value.result[idx].data='0x01';
   }
   res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify(value));
  }catch(e){res.writeHead(502);res.end(JSON.stringify({error:String(e)}));}
 });
 await new Promise(resolve=>proxy.listen(0,'127.0.0.1',resolve));
 await start(config);
 const quote=async amount=>{
  const block=await publicClient.getBlock();
  const data=encodeFunctionData({abi:routerABI,functionName:'getAmountsOut',args:[amount,[tokenA,tokenB]]});
  const raw=await rpc('eth_call',[{to:router,data},{blockHash:block.hash,requireCanonical:true}]);
  // viem's ABI decoder keeps uint256 amounts as bigint, including > MAX_SAFE_INTEGER.
  const {decodeFunctionResult}=await import('viem');
  return {amounts:decodeFunctionResult({abi:routerABI,functionName:'getAmountsOut',data:raw}),block};
 };
 const request=async(amount=100_000_000n,bps=9900n)=>{
  const q=await quote(amount);return {amount:amount.toString(),min_out:(q.amounts[1]*bps/10000n).toString(),deadline:await deadline()};
 };
 record('real_uniswap_v2_deployment',{factory,router,pair,compiler:solc.version(),router_code_hash:config.router_code_hash});

 const r=await request();
 await Promise.all(Array.from({length:8},()=>api('/intents/recover',r)));
 assert.equal(Number((await db.query("SELECT count(*) FROM intents WHERE id='recover'")).rows[0].count),1);
 await api('/intents/recover',{...r,amount:'100000001'},409);
 const before=await api('/intents/recover');
 const balanceBefore=await read(tokenB,tokenABI,'balanceOf',[trader.address]);
 dropSend=true;
 await api('/intents/recover/broadcast',{},422);
 // An actual gateway process is killed after Anvil accepted the transaction.
 await stop();await start(config);
 await rpc('anvil_mine',['0x1']);
 const recovered=await api('/intents/recover');
 assert.equal(recovered.state,'confirmed');assert.deepEqual(recovered.hashes,before.hashes);
 assert.equal(recovered.nonce,before.nonce);
 await api('/intents/recover/broadcast',{},409);
 const receipt=await mined(recovered.hashes[0]);
 const swaps=receipt.logs.filter(l=>l.address.toLowerCase()===pair.toLowerCase()).flatMap(l=>{try{return [decodeEventLog({abi:pairArtifact.abi,...l})];}catch{return [];}}).filter(l=>l.eventName==='Swap');
 assert.equal(swaps.length,1);
 const balanceAfter=await read(tokenB,tokenABI,'balanceOf',[trader.address]);
 assert(balanceAfter-balanceBefore>=BigInt(r.min_out));assert(balanceAfter-balanceBefore>BigInt(Number.MAX_SAFE_INTEGER));
 record('durable_intent_concurrency_response_loss_process_restart',{nonce:recovered.nonce,hash:recovered.hashes[0],swaps:1,atomic_output:(balanceAfter-balanceBefore).toString()});

 await rpc('evm_setAutomine',[false]);
 await api('/intents/replace',await request());
 await api('/intents/replace/broadcast',{});
 await api('/intents/replace/bump',{});
 await api('/intents/replace/broadcast',{});
 const replacing=await api('/intents/replace');assert.equal(replacing.hashes.length,2);
 await rpc('anvil_mine',['0x1']);await rpc('evm_setAutomine',[true]);
 const replaced=await api('/intents/replace');assert.equal(replaced.state,'mined');
 const winning=await mined(replaced.hashes[1]);assert.equal(winning.status,'success');
 const old=await rpc('eth_getTransactionReceipt',[replaced.hashes[0]]);assert.equal(old,null);
 await api('/intents/replace/bump',{},409);
 record('same_nonce_fee_replacement',{nonce:replaced.nonce,hashes:replaced.hashes,winner:winning.transactionHash});

 const parallelRequest=await request();
 const parallelIDs=Array.from({length:4},(_,i)=>'parallel-'+i);
 await Promise.all(parallelIDs.map(id=>api('/intents/'+id,parallelRequest)));
 const allocated=await Promise.all(parallelIDs.map(id=>api('/intents/'+id)));
 allocated.sort((a,b)=>a.nonce-b.nonce);
 assert.equal(new Set(allocated.map(x=>x.nonce)).size,4);
 assert.deepEqual(allocated.map(x=>x.nonce),Array.from({length:4},(_,i)=>replaced.nonce+1+i));
 for(const intent of allocated){await api('/intents/'+intent.id+'/broadcast',{});assert.equal((await api('/intents/'+intent.id)).state,'mined');}
 record('concurrent_distinct_nonce_allocation',{nonces:allocated.map(x=>x.nonce),all_mined:true});

 wrongChain=true;
 await api('/intents/wrong-chain',await request(),422);
 await api('/intents/recover',undefined,422);
 await api('/index/sync',{},422);
 wrongChain=false;
 await api('/intents/expired',{...await request(),deadline:1},422);
 await api('/intents/slippage',await request(100_000_000n,11000n),422);
 await api('/intents/precision',{...await request(),amount:'1.000001'},422);
 await write(traderWallet,tokenA,tokenABI,'approve',[router,0n]);
 await api('/intents/no-allowance',await request(),422);
 await write(traderWallet,tokenA,tokenABI,'approve',[router,10n**11n]);
 assert.equal(Number((await db.query("SELECT count(*) FROM intents WHERE id IN ('wrong-chain','expired','slippage','precision','no-allowance')")).rows[0].count),0);
 record('fail_closed_chain_precision_allowance_deadline_slippage',{invalid_intents:0});

 await api('/intents/market-moved',await request(100_000_000n,9999n));
 const beforeRevert=await read(tokenB,tokenABI,'balanceOf',[trader.address]);
 await write(ownerWallet,router,routerABI,'swapExactTokensForTokens',[10n**11n,1n,[tokenA,tokenB],owner.address,BigInt(await deadline())]);
 await api('/intents/market-moved/broadcast',{});
 assert.equal((await api('/intents/market-moved')).state,'reverted');
 assert.equal(await read(tokenB,tokenABI,'balanceOf',[trader.address]),beforeRevert);
 record('simulation_success_inclusion_revert',{state:'reverted',output_balance_unchanged:true});

 // Scan actual block/log/receipt data into real PostgreSQL.
 await Promise.all(Array.from({length:3},()=>api('/index/sync',{})));
 const health=await api('/health');assert.equal(health.lag,0);
 // Hold the actual writer lock. A waiting batch must not capture a stale head.
 await db.query('BEGIN');await db.query('SELECT pg_advisory_xact_lock(31337002)');
 let waitingSync;
 try{
  headRequests=0;trackHead=true;waitingSync=api('/index/sync',{});
  let blocked=false;
  for(let i=0;i<100;i++){
   blocked=(await db.query("SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=31337002 AND NOT granted) AS blocked")).rows[0].blocked;
   if(blocked)break;await pause(20);
  }
  assert.equal(blocked,true,'sync did not wait for the writer lock');
  assert.equal(headRequests,0,'sync read head before acquiring the writer lock');
  await rpc('anvil_mine',['0x1']);
 }finally{trackHead=false;await db.query('ROLLBACK');}
 await waitingSync;
 const afterWait=await api('/health');
 assert.equal(afterWait.epoch,health.epoch);assert.equal(afterWait.lag,0);
 const first=await api('/events?limit=2');assert(first.next);
 const fixedHeight=first.snapshot_height;
 await write(ownerWallet,tokenA,tokenABI,'transfer',[trader.address,1n]);
 await api('/index/sync',{});
 let page=first;const identities=[];
 for(let n=0;n<100;n++){
  assert.equal(page.snapshot_height,fixedHeight);
  identities.push(...page.events.map(e=>`${e.block_hash}/${e.transaction}/${e.index}`));
  if(!page.next)break;
  page=await api('/events?limit=2&cursor='+page.next);
  if(n===99)throw new Error('pagination never ended');
 }
 const expectedCount=Number((await db.query('SELECT count(*) FROM raw_logs l JOIN canonical c ON c.hash=l.block_hash WHERE l.height <= $1',[fixedHeight])).rows[0].count);
 assert.equal(identities.length,expectedCount);assert.equal(new Set(identities).size,identities.length);
 record('indexer_snapshot_keyset_pagination',{snapshot_height:fixedHeight,events:identities.length,new_head:(await api('/health')).indexed,head_read_after_writer_lock:true});

 const checkpoint=(await db.query('SELECT * FROM index_meta')).rows[0];
 await write(ownerWallet,tokenA,tokenABI,'transfer',[trader.address,2n]);
 for(const mode of ['omit','poison']){
  logFault=mode;await api('/index/sync',{},422);logFault='';
  assert.deepEqual((await db.query('SELECT * FROM index_meta')).rows[0],checkpoint);
 }
 assert.equal((await api('/health')).quality_errors_total,2);
 record('rpc_log_omission_and_poison',{quality_errors:2,checkpoint_unchanged:true});

 const rawCount=(await db.query('SELECT count(*) FROM raw_blocks')).rows[0].count;
 await db.query(`CREATE FUNCTION inject_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected batch failure'; END $$; CREATE TRIGGER crash_batch BEFORE INSERT ON canonical FOR EACH ROW EXECUTE FUNCTION inject_failure()`);
 await api('/index/sync',{},422);
 assert.deepEqual((await db.query('SELECT * FROM index_meta')).rows[0],checkpoint);
 assert.equal((await db.query('SELECT count(*) FROM raw_blocks')).rows[0].count,rawCount);
 await db.query('DROP TRIGGER crash_batch ON canonical; DROP FUNCTION inject_failure()');
 await api('/index/sync',{});
 record('postgres_batch_rollback_and_replay',{raw_and_checkpoint_atomic:true});

 const snapshot=await rpc('evm_snapshot');
 await api('/intents/reorg',await request());await api('/intents/reorg/broadcast',{});
 await rpc('anvil_mine',['0x1']);
 const onOld=await api('/intents/reorg');assert.equal(onOld.state,'confirmed');
 await api('/index/sync',{});
 const oldCursor=await api('/events?limit=1');
 const oldBlock=onOld.block_hash;
 const oldLogs=Number((await db.query('SELECT count(*) FROM raw_logs WHERE block_hash=$1',[oldBlock])).rows[0].count);assert(oldLogs>0);
 assert.equal(await rpc('evm_revert',[snapshot]),true);
 // Drop any rolled-back txpool entry, then create a distinct longer branch.
 await rpc('anvil_dropAllTransactions');
 await rpc('evm_increaseTime',[7]);
 await write(ownerWallet,tokenA,tokenABI,'transfer',[trader.address,3n]);
 await rpc('anvil_mine',['0x3']);
 await api('/index/sync',{});
 assert.equal((await api('/intents/reorg')).state,'orphaned_review');
 await api('/intents/reorg/broadcast',{},409);
 await api('/events?limit=1&cursor='+oldCursor.next,undefined,409);
 assert.equal(Number((await db.query('SELECT count(*) FROM raw_logs WHERE block_hash=$1',[oldBlock])).rows[0].count),oldLogs);
 assert.equal(Number((await db.query('SELECT count(*) FROM raw_logs l JOIN canonical c ON c.hash=l.block_hash WHERE l.block_hash=$1',[oldBlock])).rows[0].count),0);
 const postReorg=await api('/health');assert.equal(postReorg.epoch,1);assert.equal(postReorg.lag,0);
 // The original signed transaction can be included again by the network.
 // Record its new fact, but do not silently clear the signer review barrier.
 const replayRaw=(await db.query("SELECT raw FROM attempts WHERE intent='reorg' ORDER BY seq DESC LIMIT 1")).rows[0].raw;
 await rpc('eth_sendRawTransaction',['0x'+replayRaw.toString('hex')]);await rpc('anvil_mine',['0x2']);
 const reincluded=await api('/intents/reorg');assert.equal(reincluded.state,'confirmed');assert.equal(reincluded.review_required,true);
 await api('/intents/after-reinclusion',await request(),409);
 assert.equal(Number((await db.query("SELECT count(*) FROM intents WHERE id='after-reinclusion'")).rows[0].count),0);
 record('canonical_reorg_raw_retention_cursor_invalidation',{old_block:oldBlock,raw_logs_retained:oldLogs,epoch:postReorg.epoch,orphaned_intent:'review',reinclusion_keeps_signer_quarantined:true});

 fs.writeFileSync(path.join(output,'integration.json'),JSON.stringify({result:'passed',checks,scope:'Actual local Anvil EVM, pinned Uniswap V2 artifacts, synthetic ERC20, real PostgreSQL and Go process; no public network or real funds'},null,2)+'\n');
 if(process.env.LAB_SERVE==='1'){
  console.log(`Verified local API: ${process.env.API_URL}\nTry: curl '${process.env.API_URL}/events?limit=2'\nPress Enter or Ctrl-C to stop.`);
  process.stdin.resume();await once(process.stdin,'data');process.stdin.pause();
 }
}finally{
 await stop();
 if(proxy){proxy.closeAllConnections();await new Promise(resolve=>proxy.close(resolve));}
 await db.end();
}
