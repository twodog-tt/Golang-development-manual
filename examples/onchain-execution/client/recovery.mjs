// Local operator capability: direct access to BOTH fresh lab databases is required.
// No admin HTTP endpoint or new nonce is used. Never point this at real assets.
import fs from 'node:fs';
import {createHash,randomUUID} from 'node:crypto';
import {pathToFileURL} from 'node:url';
import pg from 'pg';
import {createPublicClient,http,keccak256,parseAbi,parseTransaction,recoverTransactionAddress,decodeFunctionData} from 'viem';
import {privateKeyToAccount} from 'viem/accounts';
import {foundry} from 'viem/chains';

const stable=v=>Array.isArray(v)?v.map(stable):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,stable(v[k])])):v;
const encode=v=>JSON.stringify(stable(v));
const digest=v=>createHash('sha256').update(encode(v)).digest('hex');
const requireThat=(condition,message)=>{if(!condition)throw new Error(message);};
const material=e=>Object.fromEntries(Object.entries(e).filter(([k])=>k!=='indexed'));
const tokenABI=parseAbi(['function balanceOf(address) view returns(uint256)','function custody() view returns(address)']);
const key=v=>v.startsWith('0x')?v:'0x'+v;
const sameAddress=(a,b)=>a?.toLowerCase()===b?.toLowerCase();
const actions=new Set(['rebind','compensate','cancel','release-signer']);

export async function openRecovery(env=process.env){
 for(const name of ['DATABASE_URL','CHAIN_DATABASE_URL','ANVIL_URL','CHAIN_GATEWAY_URL','LAB_CONFIG','LAB_SIGNER_KEY'])requireThat(env[name],`Missing local operator setting ${name}`);
 for(const name of ['ANVIL_URL','CHAIN_GATEWAY_URL','DATABASE_URL','CHAIN_DATABASE_URL'])requireThat(['127.0.0.1','localhost','[::1]'].includes(new URL(env[name]).hostname),'Recovery lab accepts loopback services only');
 const config=JSON.parse(fs.readFileSync(env.LAB_CONFIG,'utf8')),account=privateKeyToAccount(key(env.LAB_SIGNER_KEY));
 const pc=createPublicClient({chain:foundry,transport:http(env.ANVIL_URL,{retryCount:0,timeout:10000}),cacheTime:0});
 const connect=async url=>{const c=new pg.Client({connectionString:url,connectionTimeoutMillis:5000,query_timeout:20000});await c.connect();return c;};
 const ops=await connect(env.DATABASE_URL);let chain;try{chain=await connect(env.CHAIN_DATABASE_URL);}catch(e){await ops.end();throw e;}
 const rows=async(c,sql,args=[])=> (await c.query(sql,args)).rows;
 const one=async(c,sql,args=[])=> (await rows(c,sql,args))[0];
 const table=action=>['rebind','compensate'].includes(action)?'ops.recovery_plans':'recovery_plans';
 const database=action=>['rebind','compensate'].includes(action)?ops:chain;
 const call=async(route,body)=>{
  const r=await fetch(env.CHAIN_GATEWAY_URL+route,{method:body===undefined?'GET':'POST',...(body===undefined?{}:{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}),signal:AbortSignal.timeout(12000)});
  if(r.status===404)return null;requireThat(r.ok,`Evidence service unavailable (${r.status})`);return r.json();
 };
 async function checkConfig(){
  requireThat(await pc.getChainId()===31337,'Wrong chain');
  for(const [address,hash] of [[config.router,config.router_code_hash],[config.receiver,config.receiver_code_hash]])requireThat(keccak256(await pc.getCode({address}))===hash,'Contract code changed');
  requireThat(sameAddress(await pc.readContract({address:config.receiver,abi:tokenABI,functionName:'custody'}),account.address),'Signer differs from receiver custody');
 }
 async function unlock(){await chain.query('ROLLBACK');await ops.query('ROLLBACK');}
 async function capture(ownPlan=''){
  await checkConfig();await call('/index/sync',{});
  const preliminary=await rows(ops,'SELECT id FROM ops.payments ORDER BY id');requireThat(preliminary.length<=100,'Lab recovery is limited to 100 payments');
  const proofs={};
  for(const {id} of preliminary){proofs[id]={deposit:await call('/evidence/deposits/'+id),execution:await call('/evidence/executions/convert_'+id)};}
  await ops.query('BEGIN');await ops.query("SET LOCAL lock_timeout='5s'");await ops.query('SELECT pg_advisory_xact_lock(31337009)');
  await chain.query('BEGIN');await chain.query("SET LOCAL lock_timeout='5s'");
  await chain.query('SELECT pg_advisory_xact_lock(31337001)');await chain.query('SELECT pg_advisory_xact_lock(31337002)');
  const meta=await one(chain,'SELECT height,hash,epoch FROM index_meta WHERE id=1');
  const payments=await rows(ops,'SELECT id,merchant,amount::text,asset,state,sequence::text,provider_state,settlement_evidence,audit_evidence FROM ops.payments ORDER BY id');
  requireThat(payments.length===preliminary.length,'Payments changed while gathering evidence');
  const conversions=await rows(ops,'SELECT payment,intent,request,state,output::text,evidence,audit_evidence FROM ops.conversions ORDER BY payment');
  const intents=await rows(chain,'SELECT id,payload,nonce::text,state,block_hash,review_required FROM intents ORDER BY id');
  const attempts=await rows(chain,"SELECT intent,seq,hash,encode(raw,'hex') AS raw,gas_price::text FROM attempts ORDER BY intent,seq");
  const cancellations=await rows(chain,"SELECT intent,hash,nonce::text,encode(raw,'hex') AS raw,gas_price::text,plan FROM intent_cancellations ORDER BY intent");
  const retired=await rows(chain,'SELECT id,plan FROM retired_intents ORDER BY id');
  requireThat(attempts.length+cancellations.length<=200,'Too many signed attempts for this lab');
  for(const a of [...attempts,...cancellations]){
   const raw='0x'+a.raw,tx=parseTransaction(raw),i=intents.find(i=>i.id===a.intent);
   requireThat(i&&keccak256(raw)===a.hash&&String(tx.nonce)===i.nonce&&tx.chainId===31337&&sameAddress(await recoverTransactionAddress({serializedTransaction:raw}),account.address),'Signed attempt identity/nonce mismatch');
   if(a.plan){requireThat(sameAddress(tx.to,account.address)&&(tx.value??0n)===0n&&tx.gas===21000n&&(!tx.data||tx.data==='0x'),'Invalid cancellation');}
   else{
    const {args}=decodeFunctionData({abi:parseAbi(['function swapExactTokensForTokens(uint256,uint256,address[],address,uint256) returns(uint256[])']),data:tx.data});
    const request=JSON.parse(i.payload);
    requireThat(sameAddress(tx.to,config.router)&&(tx.value??0n)===0n&&String(args[0])===request.amount&&String(args[1])===request.min_out&&args[2].length===2&&sameAddress(args[2][0],config.token_in)&&sameAddress(args[2][1],config.token_out)&&sameAddress(args[3],account.address)&&String(args[4])===String(request.deadline),'Signed calldata differs from original intent');
   }
  }

  for(const p of payments){
   const proof=proofs[p.id];requireThat(proof?.deposit,'Incomplete deposit observation');
   const intent=intents.find(i=>i.id==='convert_'+p.id),fence=retired.find(i=>i.id==='convert_'+p.id);
   if(!intent && fence?.plan===ownPlan)proof.execution=null;
   requireThat(!intent || proof.execution,'Intent appeared while gathering evidence');
   for(const e of [proof.deposit,proof.execution].filter(Boolean)){
    requireThat(e.epoch===Number(meta.epoch),'Indexer epoch changed; prepare a new plan');
    if(['confirmed','reverted','cancelled'].includes(e.state)){
     const b=await one(chain,'SELECT height FROM canonical WHERE hash=$1',[e.block_hash]);
     requireThat(b && b.height===String(e.height) && BigInt(meta.height)-BigInt(e.height)+1n>=2n,'Evidence is not confirmed in the locked index');
    }
   }
  }
  // Pin custody to the committed index, then verify that anchor remains canonical.
  const blockNumber=BigInt(meta.height),anchor=await pc.getBlock({blockNumber});requireThat(anchor.hash===meta.hash,'Indexer anchor is no longer canonical');
  const custody={input:(await pc.readContract({address:config.token_in,abi:tokenABI,functionName:'balanceOf',args:[account.address],blockNumber})).toString(),output:(await pc.readContract({address:config.token_out,abi:tokenABI,functionName:'balanceOf',args:[account.address],blockNumber})).toString()};
  const latest=await pc.getTransactionCount({address:account.address,blockTag:'latest'}),pending=await pc.getTransactionCount({address:account.address,blockTag:'pending'});
  const accounts=await rows(ops,'SELECT id,currency,balance::text FROM lab.accounts ORDER BY id');
  const merchants=(await rows(ops,'SELECT DISTINCT merchant FROM ops.wallets ORDER BY merchant')).map(r=>r.merchant);
  const journals=await rows(ops,'SELECT id,payload,reversal_of FROM lab.journals ORDER BY id');
  const monitors=await rows(ops,'SELECT id,payment,monitor_started,monitor_checked FROM ops.recovery_plans WHERE applied AND needs_monitor ORDER BY id');
  const paid=await rows(chain,"SELECT l.payload FROM raw_logs l JOIN canonical c ON c.hash=l.block_hash WHERE l.kind='paid' ORDER BY l.block_hash,l.log_index");
  const drift=await one(ops,'SELECT count(*)::text AS count FROM lab.balance_drift');requireThat(drift.count==='0','Ledger projection drift');
  const unbalanced=await one(ops,'SELECT count(*)::text AS count FROM (SELECT journal_id,currency FROM lab.entries GROUP BY journal_id,currency HAVING sum(delta)<>0) q');requireThat(unbalanced.count==='0','Unbalanced asset journal');
  const snapshot={chain_id:31337,signer:account.address,contracts:config,epoch:Number(meta.epoch),payments,conversions,intents,attempts:attempts.map(({raw,...r})=>({...r,raw_hash:keccak256('0x'+raw)})),cancellations:cancellations.map(({raw,...r})=>({...r,raw_hash:keccak256('0x'+raw)})),retired:retired.filter(r=>r.plan!==ownPlan),accounts,merchants,journals,monitors,proofs:Object.fromEntries(Object.entries(proofs).map(([id,p])=>[id,{deposit:material(p.deposit),execution:p.execution?material(p.execution):null}])),custody,nonce:{latest:String(latest),pending:String(pending)},paid:paid.map(r=>r.payload)};
  await assertAnchor(meta);
  return {snapshot,proofs,meta,attempts,cancellations,retired};
 }
 async function assertAnchor(meta){requireThat((await pc.getBlock({blockNumber:BigInt(meta.height)})).hash===meta.hash,'Chain changed during verification');}
 function paymentCase(o,id){const p=o.snapshot.payments.find(p=>p.id===id);requireThat(p,'Unknown payment');return {p,c:o.snapshot.conversions.find(c=>c.payment===id),...o.proofs[id]};}
 function has(o,id){return o.snapshot.journals.some(j=>j.id===id);}
 function validDeposit(p,d){return d?.state==='confirmed'&&d.reference===p.id&&d.amount===p.amount&&sameAddress(d.token,config.token_in);}
 function admissible(action,target,o){
  const s=o.snapshot;
  if(action==='cancel'){
   const i=s.intents.find(i=>i.id===target);requireThat(i?.review_required,'Intent is not quarantined');
   requireThat(i.nonce===s.nonce.latest,'Only the next unconsumed nonce can be cancelled');
   requireThat(!s.cancellations.some(c=>c.intent===target),'Cancellation already exists; retry its original plan');
   const p=s.payments.find(p=>'convert_'+p.id===target);requireThat(p,'Payment intent required');
   requireThat(o.proofs[p.id].execution?.state==='orphaned','Original execution is not proven orphaned');
  }else if(action==='rebind'){
   const {p,c,deposit,execution}=paymentCase(o,target);
   requireThat(p.state==='review_chain'&&validDeposit(p,deposit),'Confirmed deposit and review_chain required');
   requireThat(c&&['settled','released'].includes(c.state),'Rebinding supports unreversed settled/released journals only');
   requireThat(!has(o,'reorg:'+p.id)&&!has(o,'undo-out:'+p.id)&&!has(o,'undo-in:'+p.id),'Reinstatement of compensated funds requires a separate accounting design');
   requireThat((c.state==='settled'&&execution?.state==='confirmed'&&execution.input===p.amount&&execution.amount===c.output&&sameAddress(execution.token,config.token_out))||(c.state==='released'&&execution?.state==='reverted'&&has(o,'release:'+p.id)),'Current execution does not support the recorded ledger phase/amount');
  }else if(action==='compensate'){
   const {p,c,deposit,execution}=paymentCase(o,target);
   requireThat(['settled','review_chain','review_reversed'].includes(p.state),'Payment has no recoverable financial phase');
   requireThat(validDeposit(p,deposit)||deposit.state==='orphaned','Deposit is pending or inconsistent');
   requireThat(c&&has(o,'hold:'+p.id)&&has(o,'settle:'+p.id),'Conversion and original journals required');
   requireThat(!(has(o,'reorg:'+p.id)&&deposit.state==='confirmed'),'Compensated deposits cannot be reinstated by this action');
   if(c.state==='settled')requireThat(has(o,'swap-in:'+p.id)&&has(o,'swap-out:'+p.id)&&!has(o,'undo-in:'+p.id)&&!has(o,'undo-out:'+p.id),'Settled conversion journals are not active');
   const exists=s.intents.some(i=>i.id===c.intent);
   if(!exists)requireThat(['held','executing','review','recovered_cancelled'].includes(c.state),'A historically executed intent cannot be treated as never created');
   requireThat(exists?execution?.state==='cancelled':!execution||execution.state==='retired','Require confirmed same-nonce cancellation or a fenced, never-created intent');
   if(c.state==='settled')requireThat(BigInt(s.accounts.find(a=>a.id===p.merchant+':out').balance)>=BigInt(c.output),'Insufficient output for full compensation');
   requireThat(['settled','held','executing','review','review_reversed','released','recovered_cancelled'].includes(c.state),'Unsupported financial phase');
  }else if(action==='release-signer'){
   requireThat(target==='signer','Target must be the configured signer');
   requireThat(s.nonce.latest===s.nonce.pending,'Pending transactions remain');
   requireThat(!s.payments.some(p=>['review_chain','review_reversed','review'].includes(p.state)),'Unresolved business reviews remain');
   requireThat(s.monitors.every(m=>m.monitor_started&&m.monitor_checked),'Recovery audit has not started and completed its first check');
   requireThat(s.conversions.every(c=>['settled','released','recovered_cancelled'].includes(c.state)),'Unresolved conversion phases remain');
   for(const i of s.intents){
    const p=s.payments.find(p=>'convert_'+p.id===i.id),c=s.conversions.find(c=>c.intent===i.id),e=p&&o.proofs[p.id].execution;
    requireThat(p&&c&&['confirmed','reverted','cancelled'].includes(e?.state),'An assigned nonce has no confirmed known winner');
    requireThat(BigInt(i.nonce)<BigInt(s.nonce.latest),'Nonce gap remains');
    requireThat((c.state==='settled'&&e.state==='confirmed'&&e.amount===c.output&&e.input===p.amount)||(c.state==='released'&&e.state==='reverted')||(c.state==='recovered_cancelled'&&e.state==='cancelled'),'Execution and conversion ledger disagree');
   }
   for(const c of s.conversions.filter(c=>c.state==='recovered_cancelled'&&!s.intents.some(i=>i.id===c.intent)))requireThat(s.retired.some(r=>r.id===c.intent)&&o.proofs[c.payment].execution?.state==='retired','Retirement fence is missing');
   for(const p of s.payments){
    const d=o.proofs[p.id].deposit;
    if(p.state==='settled')requireThat(validDeposit(p,d),'Settled payment lost its canonical deposit');
    if(p.state==='recovered_closed')requireThat(d.state==='orphaned','Closed payment has reappeared');
   }
   const sum=pred=>s.accounts.filter(pred).reduce((v,a)=>v+BigInt(a.balance),0n);
   const available=sum(a=>s.merchants.includes(a.id)),held=sum(a=>a.id.startsWith('held:')),out=sum(a=>s.merchants.some(m=>a.id===m+':out'));
   let unallocated=0n;
   for(const log of s.paid){const ref=log.topics[1].slice(-48),amount=BigInt(log.data),p=s.payments.find(p=>p.id===ref);
    if(!p||p.amount!==amount.toString())unallocated+=amount;
   }
   requireThat(BigInt(s.custody.input)===available+held+unallocated&&BigInt(s.custody.output)===out,'Custody and ledger liabilities do not match');
   const highest=s.intents.reduce((v,i)=>BigInt(i.nonce)>v?BigInt(i.nonce):v,-1n);
   requireThat(highest<0n||highest+1n===BigInt(s.nonce.latest),'Untracked nonce consumption or gap');
  }
 }
 async function inspect(){let o;try{o=await capture();return o.snapshot;}finally{await unlock();}}
 async function plan(action,target='signer',ttl=300){
  requireThat(actions.has(action),'Unknown recovery action');requireThat(Number.isInteger(ttl)&&ttl>=1&&ttl<=600,'TTL must be 1..600 seconds');
  try{
   const o=await capture();admissible(action,target,o);
   const document={version:1,id:randomUUID(),action,target,snapshot:o.snapshot,expires:new Date(Date.now()+ttl*1000).toISOString()};
   if(action==='cancel'){
    const fees=o.attempts.filter(a=>a.intent===target).map(a=>BigInt(a.gas_price));
    const max=fees.reduce((a,b)=>a>b?a:b,await pc.getGasPrice());
    const price=max*2n;requireThat(price<=100_000_000_000n,'Cancellation exceeds the lab fee cap');document.gas_price=price.toString();
   }
   const hash=digest(document),db=database(action),t=table(action);
   if(db===ops)await db.query(`INSERT INTO ${t}(id,digest,document,expires,payment) VALUES($1,$2,$3,$4,$5)`,[document.id,hash,document,document.expires,target]);
   else await db.query(`INSERT INTO ${t}(id,digest,document,expires) VALUES($1,$2,$3,$4)`,[document.id,hash,document,document.expires]);
   await assertAnchor(o.meta);await chain.query('COMMIT');await ops.query('COMMIT');
   return {...document,digest:hash};
  }finally{await unlock();}
 }
 async function load(document,operator,ticket){
  requireThat(/^[A-Za-z0-9_.-]{2,64}$/.test(operator??'')&&/^[A-Za-z0-9_.-]{3,100}$/.test(ticket??''),'Operator and ticket identifiers required');
  const {digest:hash,...body}=document;requireThat(actions.has(body.action)&&digest(body)===hash,'Plan was altered');
  const saved=await one(database(body.action),`SELECT * FROM ${table(body.action)} WHERE id=$1`,[body.id]);
  requireThat(saved&&saved.digest===hash&&encode(saved.document)===encode(body),'Plan is not the stored proposal');
  if(saved.applied)requireThat(saved.operator===operator&&saved.ticket===ticket,'Applied plan approval identity cannot change');
  return saved;
 }
 async function apply(document,operator,ticket){
  let saved=await load(document,operator,ticket),result;
  if(saved.applied){result={...saved.result,receipt_only:true};}
  else{
   try{
    const o=await capture(document.id),db=database(document.action),t=table(document.action);
    saved=await one(db,`SELECT * FROM ${t} WHERE id=$1 FOR UPDATE`,[document.id]);
    if(saved.applied){requireThat(saved.operator===operator&&saved.ticket===ticket,'Concurrent approval conflict');result={...saved.result,receipt_only:true};}
    else{
     requireThat(new Date(saved.expires)>new Date(),'Plan expired');
     requireThat(digest(o.snapshot)===digest(document.snapshot),'Financial facts changed; prepare a new plan');
     admissible(document.action,document.target,o);await assertAnchor(o.meta);
     result={plan:document.id,action:document.action,target:document.target,operator,ticket};
     if(document.action==='cancel'){
      const intent=o.snapshot.intents.find(i=>i.id===document.target);
      const raw=await account.signTransaction({chainId:31337,type:'legacy',nonce:Number(intent.nonce),to:account.address,value:0n,gas:21000n,gasPrice:BigInt(document.gas_price)});
      const hash=keccak256(raw);
      await chain.query('INSERT INTO intent_cancellations(intent,hash,raw,nonce,gas_price,plan) VALUES($1,$2,$3,$4,$5,$6)',[intent.id,hash,Buffer.from(raw.slice(2),'hex'),intent.nonce,document.gas_price,document.id]);
      result={...result,hash,nonce:intent.nonce,phase:'signed'};
     }else if(document.action==='release-signer'){
      // Require a fully caught-up observation before lifting the signer fence.
      requireThat((await pc.getBlock()).hash===o.meta.hash,'Indexer is behind the live head; retry');
      await chain.query('UPDATE intents SET review_required=false WHERE review_required');result.phase='released';
     }else{
      const {p,c,deposit,execution}=paymentCase(o,document.target);
      if(document.action==='rebind'){
       await ops.query("UPDATE ops.payments SET state='settled',settlement_evidence=$2 WHERE id=$1",[p.id,deposit]);
       await ops.query('UPDATE ops.conversions SET evidence=$2 WHERE payment=$1',[p.id,execution]);result.phase='settled';
      }else{
       let e=execution;
       if(!o.snapshot.intents.some(i=>i.id===c.intent)){
        // Persist the no-create fence BEFORE releasing money in the other DB.
        // If that commit is lost/aborted, the fence remains and the same plan can retry.
        const fence=await connect(env.CHAIN_DATABASE_URL);
        try{
         const r=await one(fence,'INSERT INTO retired_intents(id,plan) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET id=EXCLUDED.id RETURNING plan',[c.intent,document.id]);
         requireThat(r.plan===document.id||o.retired.some(x=>x.id===c.intent),'Intent has a different retirement owner');
        }finally{await fence.end();}
        e={reference:c.intent,state:'retired',amount:'',input:'',token:config.token_out,transaction:'',block_hash:'',height:0,indexed:Number(o.meta.height),epoch:Number(o.meta.epoch)};
       }
       const reverse=async(prefix,original)=>{if(has(o,original+p.id))await ops.query('SELECT lab.reverse_transfer($1,$2)',[prefix+p.id,original+p.id]);};
       if(c.state==='settled'){await reverse('undo-out:','swap-out:');await reverse('undo-in:','swap-in:');}
       await reverse('release:','hold:');
       if(deposit.state==='orphaned'){await reverse('reorg:','settle:');result.phase='recovered_closed';}else result.phase='settled';
       await ops.query('UPDATE ops.payments SET state=$2,settlement_evidence=$3 WHERE id=$1',[p.id,result.phase,deposit]);
       await ops.query("UPDATE ops.conversions SET state='recovered_cancelled',evidence=$2 WHERE payment=$1",[p.id,e]);
      }
      const providerState=deposit.state==='confirmed'?'confirmed':'reversed';
      const channel=await one(ops,`UPDATE ops.channel SET state=$2,evidence=$3,sequence=greatest(sequence,(SELECT sequence FROM ops.payments WHERE id=$1))+1 WHERE id=$1 RETURNING sequence::text`,[p.id,providerState,deposit]);
      requireThat(channel,'Missing original channel record');
      await ops.query('UPDATE ops.payments SET provider_state=$2,sequence=$3 WHERE id=$1',[p.id,providerState,channel.sequence]);
      await ops.query('UPDATE ops.recovery_plans SET needs_monitor=$2 WHERE id=$1',[document.id,true]);
      await ops.query('INSERT INTO ops.audit(payment,action) VALUES($1,$2)',[p.id,'operator:'+document.id]);
     }
     await db.query(`UPDATE ${t} SET applied=true,operator=$2,ticket=$3,result=$4 WHERE id=$1`,[document.id,operator,ticket,result]);
    }
    await assertAnchor(o.meta);
    // At most one of these DB transactions changes financial facts. The only
    // cross-DB prerequisite is the independent, durable retirement fence.
    await chain.query('COMMIT');await ops.query('COMMIT');
    if(env.LAB_RECOVERY_FAULT==='commit-response-lost')throw new Error('fixture: operator COMMIT response lost; retry same plan');
   }finally{await unlock();}
  }
  if(document.action==='cancel')return {...result,...await broadcastCancellation(document.id)};
  return result;
 }
 async function broadcastCancellation(planID){
  try{
   await checkConfig();await ops.query('BEGIN');await ops.query('SELECT pg_advisory_xact_lock(31337009)');await chain.query('BEGIN');await chain.query('SELECT pg_advisory_xact_lock(31337001)');
   const c=await one(chain,"SELECT intent,hash,nonce::text,encode(raw,'hex') AS raw,gas_price::text FROM intent_cancellations WHERE plan=$1",[planID]);requireThat(c,'Missing durable cancellation');
   const raw='0x'+c.raw,tx=parseTransaction(raw);
   requireThat(keccak256(raw)===c.hash&&sameAddress(await recoverTransactionAddress({serializedTransaction:raw}),account.address)&&sameAddress(tx.to,account.address)&&(tx.value??0n)===0n&&tx.gas===21000n&&(!tx.data||tx.data==='0x')&&tx.chainId===31337&&String(tx.nonce)===c.nonce,'Cancellation bytes failed validation');
   let receipt;try{receipt=await pc.getTransactionReceipt({hash:c.hash});}catch(e){if(e.name!=='TransactionReceiptNotFoundError')throw e;}
   if(receipt && (await pc.getBlock({blockNumber:receipt.blockNumber})).hash===receipt.blockHash)return {phase:'included',hash:c.hash};
   requireThat(String(await pc.getTransactionCount({address:account.address,blockTag:'latest'}))===c.nonce,'Nonce already consumed; inspect the known winner');
   try{await pc.sendRawTransaction({serializedTransaction:raw});}catch{throw new Error('Cancellation broadcast unknown; retry the same approved plan/hash');}
   return {phase:'broadcast',hash:c.hash};
  }finally{await unlock();}
 }
 return {inspect,plan,apply,close:async()=>{await unlock();await ops.end();await chain.end();}};
}

if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
 let recovery;
 try{
  const [command,...args]=process.argv.slice(2);recovery=await openRecovery();let result;
  if(command==='inspect')result=await recovery.inspect();
  else if(command==='plan'){
   const [action,target,...flags]=args,out=flags[flags.indexOf('--out')+1];
   requireThat(flags.includes('--out')&&out,'Usage: plan ACTION TARGET --out FILE');
   result=await recovery.plan(action,target);fs.writeFileSync(out,JSON.stringify(result,null,2)+'\n',{mode:0o600,flag:'wx'});
   result={plan:result.id,digest:result.digest,file:out};
  }else if(command==='apply'){
   const [file,...flags]=args;requireThat(flags.includes('--operator')&&flags.includes('--ticket'),'Usage: apply FILE --operator NAME --ticket TICKET');
   result=await recovery.apply(JSON.parse(fs.readFileSync(file,'utf8')),flags[flags.indexOf('--operator')+1],flags[flags.indexOf('--ticket')+1]);
  }else throw new Error('Usage: recovery.mjs inspect | plan rebind|compensate|cancel|release-signer TARGET --out FILE | apply FILE --operator NAME --ticket TICKET');
  console.log(JSON.stringify(result,null,2));
 }catch(e){console.error(e.message);process.exitCode=1;}finally{if(recovery)await recovery.close();}
}
