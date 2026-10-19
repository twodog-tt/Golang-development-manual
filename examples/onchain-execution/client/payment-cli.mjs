// Run from the repository root after sourcing the e2e runner's lab.env.
import {randomUUID} from 'node:crypto';
import {createPublicClient,createWalletClient,http,padHex,parseAbi} from 'viem';
import {foundry} from 'viem/chains';
import {OpsClient,fixtureKey} from '../../stablecoin-ops/client/dist/client.js';

const client=new OpsClient(process.env.API_URL,fixtureKey,process.env.PAYMENT_ASSET);
const pc=createPublicClient({chain:foundry,transport:http(process.env.ANVIL_URL),cacheTime:0});
const wallet=createWalletClient({account:client.account,chain:foundry,transport:http(process.env.ANVIL_URL)});
const tokenABI=parseAbi(['function approve(address,uint256) returns (bool)']);
const receiverABI=parseAbi(['function pay(bytes32,uint256)']);
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function wait(read,accept){for(let i=0;i<200;i++){const v=await read();if(accept(v))return v;await sleep(300);}throw new Error('Timed out; inspect original payment/intent, do not create a new order');}
async function pay(id){const p=await client.get(id);for(const [address,abi,functionName,args] of [
 [process.env.CHAIN_TOKEN_IN,tokenABI,'approve',[process.env.LAB_RECEIVER,BigInt(p.amount_atomic)]],
 [process.env.LAB_RECEIVER,receiverABI,'pay',[padHex('0x'+id,{size:32}),BigInt(p.amount_atomic)]],
 ]){const hash=await wallet.writeContract({address,abi,functionName,args});const receipt=await pc.waitForTransactionReceipt({hash});if(receipt.status!=='success')throw new Error('Onchain payment failed: '+hash);}}
async function convert(id){const q=await client.call('/payments/'+id+'/conversion-quote');return client.call('/payments/'+id+'/convert','POST',{min_out:q.min_out,deadline:q.deadline});}
await client.login();
const [command,...args]=process.argv.slice(2);
let result;
switch(command){
case 'create':result=await client.create(args[0],args[1]);break;
case 'approve':result=await client.approve(args[0]);break;
case 'pay':await pay(args[0]);result={payment:args[0],submitted:true};break;
case 'convert':result=await convert(args[0]);break;
case 'show':{
 const payment=await client.get(args[0]);const response=await client.request('/payments/'+args[0]+'/conversion');
 result={payment,conversion:response.ok?await response.json():null};break;
}
case 'ops':result=await client.call('/ops');break;
case 'demo':{
 const payment=await client.create('cli-'+randomUUID(),args[0]??'10');await client.approve(payment.id);
 await wait(()=>client.get(payment.id),p=>p.state==='pending');await pay(payment.id);
 await wait(()=>client.get(payment.id),p=>p.state==='settled');await convert(payment.id);
 const conversion=await wait(()=>client.call('/payments/'+payment.id+'/conversion'),c=>['settled','released','review','review_reversed'].includes(c.state));
 result={payment:await client.get(payment.id),conversion};break;
}
default:throw new Error('Usage: payment-cli.mjs demo AMOUNT | create KEY AMOUNT | approve ID | pay ID | convert ID | show ID | ops');
}
console.log(JSON.stringify(result,null,2));
