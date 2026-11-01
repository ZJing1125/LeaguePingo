import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { findBrowser, findTool, findNSIS, describeEnvironment } from './environment.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
test('tool lookup respects host PATH separators rather than machine-specific drives', () => {
  const candidate = path.join('/tools', 'go');
  assert.equal(findTool('go', { PATH: '/missing:/tools' }, 'linux', p => p === candidate), candidate);
  const win = path.join('Q:/Programs', 'node.exe');
  assert.equal(findTool('node', { PATH: 'Q:/Missing;Q:/Programs' }, 'win32', p => p === win), win);
  assert.equal(findTool('wails3', {}, 'linux', () => false), null);
});
test('browser discovery honors explicit configuration and fails closed when missing', () => {
  assert.equal(findBrowser({ BROWSER_EXECUTABLE: '/custom/browser' }, 'linux', () => true), '/custom/browser');
  assert.throws(() => findBrowser({ BROWSER_EXECUTABLE: '/missing' }, 'linux', () => false), /does not exist/);
  const expected = path.join('Q:/Applications', 'Microsoft/Edge/Application/msedge.exe');
  assert.equal(findBrowser({ ProgramFiles: 'Q:/Applications' }, 'win32', p => p === expected), expected);
  assert.throws(() => findBrowser({}, 'linux', () => false), /No local browser/);
});
test('NSIS discovery is explicit and does not install packaging tools', () => {
  assert.equal(findNSIS({ LEAGUEPINGO_MAKENSIS: '/tools/makensis' }, 'linux', p => p === '/tools/makensis'), '/tools/makensis');
  const expected = path.join('Q:/Programs', 'NSIS', 'makensis.exe');
  assert.equal(findNSIS({ ProgramFiles: 'Q:/Programs', PATH: '' }, 'win32', p => p === expected), expected);
  assert.throws(() => findNSIS({ PATH: '' }, 'linux', () => false), /NSIS not found/);
});
test('capability report distinguishes host from product and does not reveal secrets', () => {
  const report = describeEnvironment({ PATH: '', PRIVATE_TOKEN: 'do-not-print' }, 'linux', 'arm64');
  assert.equal(report.windowsDeliveryHost, false);
  assert.equal(report.portableToolsPresent, false);
  assert.ok(!JSON.stringify(report).includes('do-not-print'));
});
test('optional local environment loads explicitly through the shell bootstrap', t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-env-test-'));
  t.after(() => {
    assert.ok(path.basename(dir).startsWith('leaguepingo-env-test-'));
    fs.rmSync(dir, { recursive: true, force: true });
  });
  fs.mkdirSync(path.join(dir, 'scripts'));
  fs.copyFileSync(path.join(root, 'scripts/env.sh'), path.join(dir, 'scripts/env.sh'));
  const env = { ...process.env };
  delete env.LEAGUEPINGO_BASH;
  delete env.LEAGUEPINGO_ENV_TEST;
  const run = () => spawnSync(process.env.LEAGUEPINGO_BASH || 'bash', ['-c',
    'task_root=$1; source "$task_root/scripts/env.sh"; test -n "$LEAGUEPINGO_BASH"; printf "%s" "${LEAGUEPINGO_ENV_TEST:-absent}"',
    'env-test', dir.replaceAll('\\', '/')], { env, encoding: 'utf8' });
  assert.equal(run().stdout, 'absent');
  fs.mkdirSync(path.join(dir, '.leaguepingo'));
  fs.writeFileSync(path.join(dir, '.leaguepingo/environment.sh'), 'export LEAGUEPINGO_ENV_TEST=local-only\n');
  const configured = run();
  assert.equal(configured.status, 0, configured.stderr);
  assert.equal(configured.stdout, 'local-only');
});
test('full verification cannot inherit portable mode and every entry loads local config', () => {
  const verify = fs.readFileSync(path.join(root, 'scripts/verify.sh'), 'utf8');
  assert.match(verify, /task_scope=\$\{1:-windows\}/);
  assert.match(verify, /windows\) export LEAGUEPINGO_VERIFY_SCOPE=windows/);
  assert.match(verify, /portable\) export LEAGUEPINGO_VERIFY_SCOPE=portable/);
  assert.match(verify, /NOT RUN:/);
  for (const file of ['build.sh', 'verify.sh', 'run.sh', 'dev.sh']) {
    assert.ok(fs.readFileSync(path.join(root, 'scripts', file), 'utf8').includes('source "$task_root/scripts/env.sh"'));
  }
  const agents = fs.readFileSync(path.join(root, 'AGENTS.md'), 'utf8');
  assert.doesNotMatch(agents, /[A-Z]:\/|\/c\/Users\//);
  assert.match(agents, /AGENTS\.local\.md/);
});
