// Exercise the site's actual search worker and language assets without a browser.
// Run after mkdocs build: node scripts/verify_search.cjs [site-directory]
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const root = path.resolve(process.argv[2] || 'site');
const workerDir = path.join(root, 'assets/javascripts/workers');
const workers = fs.readdirSync(workerDir).filter(name => /^search\..*\.min\.js$/.test(name));
if (workers.length !== 1) throw new Error('Expected exactly one built search worker');
const worker = path.join(workerDir, workers[0]);
const index = JSON.parse(fs.readFileSync(path.join(root, 'search/search_index.json'), 'utf8'));
let handler, resolve;
const sandbox = {
  console,
  addEventListener: (type, fn) => { if (type === 'message') handler = fn; },
  postMessage: message => resolve(message),
};
sandbox.self = sandbox;
const context = vm.createContext(sandbox);
sandbox.importScripts = (...urls) => {
  for (const url of urls) {
    const file = path.resolve(workerDir, url);
    if (!file.startsWith(path.join(root, 'assets/javascripts/lunr/'))) {
      throw new Error('Unexpected search language asset path');
    }
    vm.runInContext(fs.readFileSync(file, 'utf8'), context, {filename: file, timeout: 20000});
  }
};
vm.runInContext(fs.readFileSync(worker, 'utf8'), context, {filename: worker, timeout: 20000});
const send = message => new Promise((done, reject) => {
  resolve = done;
  Promise.resolve(handler({data: message})).catch(reject);
});

(async () => {
  const setup = await send({type: 0, data: {...index, options: {suggest: true}}});
  if (setup.type !== 1) throw new Error('Search worker did not initialize');
  const article = 'S-SEC-03-sbom-provenance-release-admission/';
  const queries = [
    ['SBOM', article],
    ['发布准入', article],
    ['Provenance', article],
    ['provenance', article],
    ['resolvedDependencies', article],
    ['Outbox', 'S-DIST-06-outbox-inbox-fault-lab/'],
    ['双分录', 'S-PAY-07-postgresql-double-entry-ledger-lab/'],
  ];
  const checks = [];
  for (const [query, target] of queries) {
    const result = await send({type: 2, data: query});
    if (result.type !== 3 || !result.data.items.flat().some(row => row.location.includes(target))) {
      throw new Error('Expected article not found for query: ' + query);
    }
    checks.push({query, found: true});
  }
  const empty = await send({type: 2, data: 'zzzxqvnomatch987'});
  if (empty.type !== 3 || empty.data.items.length) throw new Error('No-match query returned results');
  console.log(JSON.stringify({passed: true, engine: workers[0], checks, emptySearch: true}, null, 2));
})().catch(error => { console.error(error); process.exitCode = 1; });
