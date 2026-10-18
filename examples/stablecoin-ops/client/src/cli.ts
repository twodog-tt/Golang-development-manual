import { OpsClient } from './client.js';
const client = new OpsClient(process.env.API_URL ?? 'http://127.0.0.1:8081');
await client.login(); // token stays in this process and is never printed or persisted.
const [command, value, amount] = process.argv.slice(2);
let result: unknown;
switch(command) {
  case 'create': result = await client.create(value,amount); break;
  case 'get': result = await client.get(value); break;
  case 'approve': result = await client.approve(value); break;
  case 'refund': result = await client.refund(value); break;
  case 'reconcile': result = await client.call('/payments/' + value + '/reconcile','POST'); break;
  case 'notify': result = await client.call('/payments/' + value + '/notify','POST'); break;
  case 'ops': result = await client.call('/ops'); break;
  default: throw new Error('Usage: npm run ops -- create KEY AMOUNT | get ID | approve ID | refund ID | ops');
}
console.log(JSON.stringify(result,null,2));
