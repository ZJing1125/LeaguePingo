// Fixed coverage codes only: unknown reasons must not leak backend text.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const ts = require('typescript');
function load(file) {
  const filename = path.resolve(__dirname, '../src', file + '.ts');
  const mod = new Module(filename, module);
  mod.filename = filename;
  mod.require = () => { throw new Error('scoutCoverage must stay dependency free'); };
  mod._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, filename);
  return mod.exports;
}
const { scoutCoverageText, scoutReason, scoutRetrySeconds } = load('scoutCoverage');
assert.equal(scoutReason('RequestTimeout'), '战绩服务响应超时');
assert.equal(scoutReason('range_limited'), '当前记录范围受限');
assert.equal(scoutReason('some_new_code'), '战绩数据暂不可用');
assert.equal(scoutReason(undefined), '');
const now = Date.parse('2026-11-01T00:00:00Z');
assert.equal(scoutRetrySeconds('2026-11-01T00:00:30Z', now), 30);
assert.equal(scoutRetrySeconds('2026-10-31T23:00:00Z', now), null);
assert.equal(scoutRetrySeconds('not-a-time', now), null);
assert.equal(scoutCoverageText({ reason: 'RequestTimeout', retryAt: '2026-11-01T00:00:30Z' }, now), '战绩服务响应超时，约 30 秒后自动重试');
assert.equal(scoutCoverageText({ reason: 'ServiceBusy' }, now), '战绩服务暂时繁忙');
assert.equal(scoutCoverageText({}, now), '统计尚未读全，可在主程序刷新数据');
assert.equal(scoutCoverageText(null, now), '统计尚未读全，可在主程序刷新数据');
console.log('PASS: scout coverage codes, retry countdown and safe generic fallback.');
