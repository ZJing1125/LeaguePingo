// Production formatter and the same catalog embedded into Go. No browser/API calls.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const ts = require('typescript');
const filename = path.resolve(__dirname, '../src/errors.ts');
const mod = new Module(filename, module);
mod.filename = filename;
mod.paths = module.paths;
mod._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true } }).outputText, filename);
const {errorMessage,retryTime} = mod.exports;
const messages = require('../../internal/apperrors/messages.json');
for (const [code, message] of Object.entries(messages)) {
  const err = new Error('LCU HTTP 503 https://private-host/player#DEMO?token=secret');
  err.name = 'RuntimeError';
  err.cause = {code, message: 'hostile response'};
  assert.equal(errorMessage(err), message);
  assert.equal(errorMessage({code, message: 'do not render'}), message);
}
for (const reason of [undefined, null, 500, '<html>secret</html>', '看似安全的中文私密玩家#DEMO',
  new Error('Get https://private-host/player#DEMO: EOF'),
  {message:'HTTP 429'}, {cause:{code:'__proto__',message:'secret'}},
  {cause:{code:'toString'}}, {cause:{code:'Unknown',message:'private-player'}},
  {cause:{message:messages.ServiceBusy}}, {cause:[{code:'ServiceBusy'}]}]) {
  assert.equal(errorMessage(reason), messages.Unexpected, 'untrusted text became UI content');
}
console.log('PASS: shared error catalog, Wails RuntimeError cause, hostile/unknown errors and fixed Chinese fallback.');
const at='2026-10-29T12:00:00Z';
assert.equal(retryTime({cause:{code:'RateLimited',retryAt:at}}),Date.parse(at));
for(const reason of [null,new Error('HTTP 503'),{cause:{code:'AccessDenied',retryAt:at}},{cause:{code:'ServiceBusy',retryAt:'invalid'}},{cause:{code:'RequestTimeout'}}])assert.equal(retryTime(reason),0);
