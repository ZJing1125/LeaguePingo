import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { createHash } from 'node:crypto';
import { prepareDownload } from './ci-download.mjs';

const identity = { commit: 'a'.repeat(40), repository: 'example/project', runID: '123', attempt: '2' };
function fixture(t, commit = 'aaaaaaa') {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-ci-download-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  fs.mkdirSync(path.join(root, 'bin'));
  fs.mkdirSync(path.join(root, 'artifacts/installer-check'), { recursive: true });
  fs.writeFileSync(path.join(root, 'VERSION'), '0.2.0-dev');
  fs.writeFileSync(path.join(root, 'bin/leaguepingo-ci.exe'), 'synthetic exe');
  fs.writeFileSync(path.join(root, 'artifacts/installer-check/leaguepingo-ci-installer.exe'), 'synthetic installer');
  const sha256 = createHash('sha256').update('synthetic exe').digest('hex');
  fs.writeFileSync(path.join(root, 'bin/artifacts.json'), JSON.stringify({ version: 1, retired: [], builds: [
    { file: 'bin/leaguepingo-ci.exe', sha256, commit, accepted: false, evidence: [], builtAt: '2026-11-16T00:00:00Z' },
  ] }));
  return root;
}
test('download contains matching executables, checksums and portable receipt, never runner state', t => {
  const root = fixture(t), result = prepareDownload(root, identity);
  assert.deepEqual(fs.readdirSync(result.folder).sort(), ['BUILD-INFO.json', 'README.txt', 'SHA256SUMS.txt', 'leaguepingo-setup.exe', 'leaguepingo.exe']);
  const raw = fs.readFileSync(path.join(result.folder, 'BUILD-INFO.json'), 'utf8');
  const receipt = JSON.parse(raw);
  assert.equal(receipt.commit, identity.commit);
  assert.equal(receipt.realClientAcceptance, 'pending');
  assert.equal(receipt.runAttempt, 2);
  assert(!raw.includes(root) && !raw.includes('evidence') && !raw.includes('accepted'));
  for (const line of fs.readFileSync(path.join(result.folder, 'SHA256SUMS.txt'), 'utf8').trim().split('\n')) {
    const [hash, name] = line.split('  ');
    assert.equal(hash, createHash('sha256').update(fs.readFileSync(path.join(result.folder, name))).digest('hex'));
  }
  assert.throws(() => prepareDownload(root, identity), /EEXIST/);
});
test('changed, dirty or wrong-commit binaries cannot be offered as the current CI build', t => {
  for (const commit of ['bbbbbbb', 'aaaaaaa-dirty']) assert.throws(() => prepareDownload(fixture(t, commit), identity), /clean current CI commit/);
  const root = fixture(t);
  fs.appendFileSync(path.join(root, 'bin/leaguepingo-ci.exe'), 'changed');
  assert.throws(() => prepareDownload(root, identity), /Changed file/);
  assert.throws(() => prepareDownload(fixture(t), { ...identity, commit: 'invalid' }), /Invalid CI/);
  const missing = fixture(t);
  fs.unlinkSync(path.join(missing, 'artifacts/installer-check/leaguepingo-ci-installer.exe'));
  assert.throws(() => prepareDownload(missing, identity), /ENOENT/);
});
