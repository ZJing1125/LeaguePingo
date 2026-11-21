import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { manager } from './artifacts.mjs';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

function fixture(t, statuses = [true, true, false], running = () => []) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-artifacts-test-'));
  t.after(() => {
    // Only this exact mkdtemp-owned directory; no shell/path interpolation.
    assert.ok(path.basename(root).startsWith('leaguepingo-artifacts-test-'));
    fs.rmSync(root, { recursive: true, force: true });
  });
  fs.mkdirSync(path.join(root, 'bin'));
  const builds = statuses.map((accepted, i) => {
    const file = `bin/leaguepingo-v${i + 1}.exe`;
    const image = `bin/ui-v${i + 1}.png`;
    fs.writeFileSync(path.join(root, file), `binary-${i}`);
    fs.writeFileSync(path.join(root, image), `image-${i}`);
    return { file, accepted, evidence: [image] };
  });
  const m = manager(root, () => running(root));
  m.bootstrap({ builds, retired: [] });
  return { root, m, exists: f => fs.existsSync(path.join(root, f)) };
}
function pe(root, file) {
  const data = Buffer.alloc(256);
  data.write('MZ'); data.writeUInt32LE(128, 60); data.set([80, 69, 0, 0], 128);
  fs.writeFileSync(path.join(root, file), data);
}

test('default preview is read-only; applied cleanup retains current and accepted fallback', t => {
  const { root, m, exists } = fixture(t);
  const before = fs.readFileSync(path.join(root, 'bin/artifacts.json'));
  const preview = m.clean();
  assert.deepEqual(preview.files, ['bin/leaguepingo-v1.exe', 'bin/ui-v1.png']);
  assert.ok(exists('bin/leaguepingo-v1.exe'));
  assert.deepEqual(fs.readFileSync(path.join(root, 'bin/artifacts.json')), before);
  m.clean(true);
  assert.ok(!exists('bin/leaguepingo-v1.exe') && !exists('bin/ui-v1.png'));
  assert.ok(exists('bin/leaguepingo-v2.exe') && exists('bin/leaguepingo-v3.exe'));
  assert.equal(m.clean(true).files.length, 0);
});

test('replacement candidate does not evict last accepted version; acceptance rotates it', t => {
  const { m } = fixture(t, [true, true, false, false]);
  assert.deepEqual(m.clean().retained, ['bin/leaguepingo-v4.exe', 'bin/leaguepingo-v2.exe']);
  m.accept('bin/leaguepingo-v4.exe');
  m.clean(true);
  assert.deepEqual(m.read().builds.map(b => b.file), ['bin/leaguepingo-v2.exe', 'bin/leaguepingo-v4.exe']);
  assert.throws(() => m.accept('bin/leaguepingo-v2.exe'), /Only the current/);
});

test('running obsolete build and its evidence survive; later cleanup removes them', t => {
  let busy = true;
  const { m, exists } = fixture(t, undefined, root => busy ? [path.join(root, 'bin/leaguepingo-v1.exe')] : []);
  assert.equal(m.clean(true).skippedRunning.length, 1);
  assert.ok(exists('bin/ui-v1.png'));
  busy = false;
  m.clean(true);
  assert.ok(!exists('bin/ui-v1.png'));
});

test('reconciliation previews deleted builds and missing evidence, preserves survivors and unknown files', t => {
  const { root, m, exists } = fixture(t, [true, false]);
  m.migrateEvidence();
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v1.exe'));
  fs.unlinkSync(path.join(root, 'artifacts/leaguepingo-v2/ui-v2.png'));
  fs.writeFileSync(path.join(root, 'artifacts/leaguepingo-v1/settings.json'), '{}');
  const manifest = path.join(root, 'bin/artifacts.json');
  const before = fs.readFileSync(manifest);
  const survivor = m.read().builds[1];
  assert.throws(() => m.audit(true)); // The former build entry failed here.
  const preview = m.reconcile();
  assert.deepEqual(preview.removedBuilds, ['bin/leaguepingo-v1.exe']);
  assert.deepEqual(preview.missingEvidence, ['artifacts/leaguepingo-v2/ui-v2.png']);
  assert.deepEqual(preview.files, ['artifacts/leaguepingo-v1/ui-v1.png']);
  assert.deepEqual(fs.readFileSync(manifest), before);
  assert.ok(exists(preview.files[0]));
  m.reconcile(true);
  assert.deepEqual(m.read().builds, [{ ...survivor, evidence: [] }]);
  assert.ok(exists('bin/leaguepingo-v2.exe'));
  assert.ok(!exists(preview.files[0]));
  assert.ok(exists('artifacts/leaguepingo-v1/settings.json'));
  assert.equal(m.audit().current.accepted, false);
  const repaired = fs.readFileSync(manifest);
  assert.deepEqual(m.reconcile(true).removedBuilds, []);
  assert.deepEqual(fs.readFileSync(manifest), repaired);
});

test('reconciliation handles deleted current/all builds and missing retired evidence', t => {
  const { root, m, exists } = fixture(t, [true, false]);
  const state = m.read();
  state.retired.push(state.builds[1].evidence.pop());
  fs.writeFileSync(path.join(root, 'bin/artifacts.json'), JSON.stringify(state));
  fs.unlinkSync(path.join(root, 'bin/ui-v2.png'));
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v2.exe'));
  m.reconcile(true);
  assert.equal(m.audit().current.file, 'bin/leaguepingo-v1.exe');
  assert.deepEqual(m.read().retired, []);
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v1.exe'));
  m.reconcile(true);
  assert.deepEqual(m.read().builds, []);
  assert.equal(m.audit(true).current, null);
  assert.ok(!exists('bin/ui-v1.png'));
  fs.unlinkSync(path.join(root, 'bin/artifacts.json'));
  assert.deepEqual(m.reconcile(true).removedBuilds, []);
  assert.ok(!exists('bin/artifacts.json'), 'a fresh checkout is still initialized by prepare');
});

test('reconciliation refuses running paths, failed process discovery and changed orphan evidence before writes', t => {
  const { root, m, exists } = fixture(t);
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v1.exe'));
  const manifest = path.join(root, 'bin/artifacts.json');
  const before = fs.readFileSync(manifest);
  const busy = manager(root, () => [path.join(root, 'bin/leaguepingo-v1.exe')]);
  assert.throws(() => busy.reconcile(true), /running/i);
  const unavailable = manager(root, () => { throw new Error('process discovery failed'); });
  assert.throws(() => unavailable.reconcile(true), /process discovery failed/);
  fs.appendFileSync(path.join(root, 'bin/ui-v1.png'), 'changed');
  assert.throws(() => m.reconcile(true), /Changed file/);
  assert.deepEqual(fs.readFileSync(manifest), before);
  assert.ok(exists('bin/ui-v1.png') && exists('bin/leaguepingo-v2.exe'));
});

test('reconciliation preflights pending manifest writes and a deleted executable reappearing', t => {
  const { root, m, exists } = fixture(t);
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v1.exe'));
  fs.writeFileSync(path.join(root, 'bin/artifacts-next.json'), 'pending write');
  const manifest = path.join(root, 'bin/artifacts.json');
  const before = fs.readFileSync(manifest);
  assert.throws(() => m.reconcile(true), /manifest|exists/i);
  assert.ok(exists('bin/ui-v1.png'));
  assert.deepEqual(fs.readFileSync(manifest), before);
  fs.unlinkSync(path.join(root, 'bin/artifacts-next.json'));
  const reappeared = manager(root, () => {
    fs.writeFileSync(path.join(root, 'bin/leaguepingo-v1.exe'), 'replacement');
    return [];
  });
  assert.throws(() => reappeared.reconcile(true), /reappeared/i);
  assert.ok(exists('bin/ui-v1.png'));
  assert.deepEqual(fs.readFileSync(manifest), before);
});

test('process discovery failure, modified old files, and missing survivors fail closed', t => {
  const { root, m, exists } = fixture(t);
  const unavailable = manager(root, () => { throw new Error('process discovery failed'); });
  assert.throws(() => unavailable.clean(true), /process discovery failed/);
  fs.appendFileSync(path.join(root, 'bin/ui-v1.png'), 'changed');
  assert.throws(() => m.clean(true), /Changed file/);
  assert.ok(exists('bin/leaguepingo-v1.exe'));
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v2.exe'));
  assert.throws(() => m.clean(true));
  assert.ok(exists('bin/leaguepingo-v1.exe'));
});

test('unknown files and isolated settings are never swept', t => {
  const { root, m, exists } = fixture(t);
  fs.writeFileSync(path.join(root, 'bin/leaguepingo-unknown.exe'), 'unknown');
  fs.mkdirSync(path.join(root, 'bin/performance-old/isolated-settings'), { recursive: true });
  fs.writeFileSync(path.join(root, 'bin/performance-old/isolated-settings/settings.json'), '{}');
  m.clean(true);
  assert.ok(exists('bin/leaguepingo-unknown.exe'));
  assert.ok(exists('bin/performance-old/isolated-settings/settings.json'));
});

test('paths, links, duplicate manifest entries and changed survivors are rejected', t => {
  const { root, m } = fixture(t);
  for (const f of ['bin/../README.md', '../bin/a.exe', 'C:/data.exe', 'bin', 'bin//x.exe', 'bin/a:stream']) {
    assert.throws(() => m.safe(f));
  }
  fs.mkdirSync(path.join(root, 'outside'));
  fs.symlinkSync(path.join(root, 'outside'), path.join(root, 'bin/link'), 'junction');
  assert.throws(() => m.safe('bin/link/file.png'), /Link/);
  const state = m.read();
  state.builds.push(state.builds[0]);
  fs.writeFileSync(path.join(root, 'bin/artifacts.json'), JSON.stringify(state));
  assert.throws(() => m.clean(true), /Duplicate/);
});

test('prepare refuses overwrite; registration validates PE; failed build leaves existing versions intact', t => {
  const { root, m, exists } = fixture(t, [true, true]);
  assert.throws(() => m.prepare('bin/leaguepingo-v2.exe'), /already exists/);
  const file = 'bin/leaguepingo-v3.exe';
  const folder = m.prepare(file);
  fs.writeFileSync(path.join(folder, 'ui.png'), 'screenshot');
  fs.writeFileSync(path.join(folder, 'review.html'), '<!doctype html><title>synthetic review</title>');
  fs.writeFileSync(path.join(root, file), 'partial');
  assert.throws(() => m.register(file, 'abcdef1'), /PE/);
  m.discard(file);
  assert.ok(!exists(file));
  assert.ok(!exists('artifacts/leaguepingo-v3'));
  assert.ok(exists('bin/leaguepingo-v1.exe') && exists('bin/leaguepingo-v2.exe'));
  const registeredFolder = m.prepare(file); pe(root, file);
  fs.writeFileSync(path.join(registeredFolder, 'review.html'), '<!doctype html><title>synthetic review</title>');
  m.register(file, 'abcdef1');
  assert.deepEqual(m.read().builds.at(-1).evidence.map(item => item.file),
    ['artifacts/leaguepingo-v3/review.html']);
  assert.throws(() => m.discard(file), /registered/);
  m.clean(true);
  assert.deepEqual(m.read().builds.map(b => b.file), ['bin/leaguepingo-v2.exe', file]);
});

test('evidence migration preserves hash and unknown legacy settings', t => {
  const { root, m, exists } = fixture(t, [true, true]);
  const before = createHash('sha256').update(fs.readFileSync(path.join(root, 'bin/ui-v2.png'))).digest('hex');
  assert.equal(m.migrateEvidence(), 2);
  const item = m.read().builds[1].evidence[0];
  assert.equal(item.file, 'artifacts/leaguepingo-v2/ui-v2.png');
  assert.equal(item.sha256, before);
  assert.ok(exists(item.file) && !exists('bin/ui-v2.png'));
});

test('fresh checkout initializes once; missing manifest with existing binaries refuses takeover', t => {
  const { root, m } = fixture(t, []);
  fs.unlinkSync(path.join(root, 'bin/artifacts.json'));
  const file = 'bin/leaguepingo-first.exe';
  m.prepare(file);
  pe(root, file);
  m.register(file, 'abcdef1');
  assert.deepEqual(m.clean(true).retained, [file]);
  fs.appendFileSync(path.join(root, file), 'tampered');
  assert.throws(() => m.clean(true), /Changed file/);
  fs.unlinkSync(path.join(root, 'bin/artifacts.json'));
  assert.throws(() => m.prepare('bin/leaguepingo-next.exe'), /explicit manifest initialization/);
});

test('later evidence is explicitly registered and follows its build; unknown settings remain', t => {
  const { root, m, exists } = fixture(t, [true, true]);
  m.migrateEvidence();
  const folder = 'artifacts/leaguepingo-v1';
  fs.writeFileSync(path.join(root, folder, 'later.csv'), 'synthetic');
  fs.writeFileSync(path.join(root, folder, 'later.html'), '<!doctype html><title>synthetic</title>');
  fs.writeFileSync(path.join(root, folder, 'settings.json'), '{}');
  m.attachEvidence('bin/leaguepingo-v1.exe');
  assert.equal(m.read().builds[0].evidence.length, 3);
  m.prepare('bin/leaguepingo-v3.exe'); pe(root, 'bin/leaguepingo-v3.exe');
  m.register('bin/leaguepingo-v3.exe', 'abcdef1');
  m.clean(true);
  assert.ok(!exists(`${folder}/later.csv`));
  assert.ok(!exists(`${folder}/later.html`));
  assert.ok(exists(`${folder}/settings.json`));
});

test('CLI write approval and exclusive lock cannot be bypassed by failed invocation', t => {
  const { root, exists } = fixture(t);
  fs.mkdirSync(path.join(root, 'scripts'));
  fs.copyFileSync(fileURLToPath(new URL('./artifacts.mjs', import.meta.url)), path.join(root, 'scripts/artifacts.mjs'));
  const cli = (...args) => spawnSync(process.execPath, ['scripts/artifacts.mjs', ...args], { cwd: root, encoding: 'utf8' });
  assert.notEqual(cli('accept', '--file', 'bin/leaguepingo-v3.exe').status, 0);
  fs.unlinkSync(path.join(root, 'bin/ui-v2.png'));
  const before = fs.readFileSync(path.join(root, 'bin/artifacts.json'));
  const preview = cli('reconcile');
  assert.equal(preview.status, 0, preview.stderr);
  assert.deepEqual(JSON.parse(preview.stdout).missingEvidence, ['bin/ui-v2.png']);
  assert.deepEqual(fs.readFileSync(path.join(root, 'bin/artifacts.json')), before);
  fs.writeFileSync(path.join(root, 'bin/artifacts.lock'), 'another process');
  assert.notEqual(cli('accept', '--file', 'bin/leaguepingo-v3.exe', '--apply').status, 0);
  assert.notEqual(cli('reconcile', '--apply').status, 0);
  assert.equal(fs.readFileSync(path.join(root, 'bin/artifacts.lock'), 'utf8'), 'another process');
  assert.deepEqual(fs.readFileSync(path.join(root, 'bin/artifacts.json')), before);
  assert.ok(exists('bin/leaguepingo-v1.exe'));
});

const nativeBuild = { skip: process.platform !== 'win32' || process.env.LEAGUEPINGO_VERIFY_SCOPE === 'portable' };
function buildFixture(t) {
  const { root, m, exists } = fixture(t, [true, true]);
  fs.mkdirSync(path.join(root, 'scripts'));
  fs.mkdirSync(path.join(root, 'test-tools'));
  for (const file of ['artifacts.mjs', 'build.sh', 'env.sh']) {
    fs.copyFileSync(fileURLToPath(new URL(`./${file}`, import.meta.url)), path.join(root, 'scripts', file));
  }
  // This fixture isolates build/retention control flow; policy routing itself is
  // covered by workflow.test.mjs with the real repository files and Wails runner.
  fs.writeFileSync(path.join(root, 'scripts/workflow.mjs'), 'process.exit(0);\n');
  fs.writeFileSync(path.join(root, 'scripts/version.mjs'), `
    if (process.argv[2] === 'value') console.log('0.2.0-dev');
  `);
  fs.writeFileSync(path.join(root, 'test-tools/git'), '#!/bin/bash\nif [[ "$1" == rev-parse ]]; then echo abcdef1; fi\n');
  fs.writeFileSync(path.join(root, 'test-tools/go'), '#!/bin/bash\nnode scripts/fake-compiler.mjs "$@"\n');
  fs.chmodSync(path.join(root, 'test-tools/git'), 0o700);
  fs.chmodSync(path.join(root, 'test-tools/go'), 0o700);
  fs.writeFileSync(path.join(root, 'scripts/fake-compiler.mjs'), `
    import fs from 'node:fs';
    const file = process.argv[process.argv.indexOf('-o') + 1];
    if (fs.existsSync('fail-compiler')) { fs.writeFileSync(file, 'partial'); process.exit(7); }
    const buffer = Buffer.alloc(256); buffer.write('MZ'); buffer.writeUInt32LE(128, 60); buffer.set([80,69,0,0],128);
    fs.writeFileSync(file, buffer);
  `);
  const binary = 'bin/leaguepingo-v3.exe';
  const bash = process.env.LEAGUEPINGO_BASH || 'bash';
  const nodeFolder = path.dirname(process.execPath).replaceAll('\\', '/');
  const run = (output = binary) => spawnSync(bash, ['-c', `export PATH="$PWD/test-tools:${nodeFolder}:$PATH"; bash scripts/build.sh ${output}`],
    { cwd: root, encoding: 'utf8', timeout: 30000 });
  return { root, m, exists, binary, run };
}

test('build shell: test/compiler failure preserves old builds; success registers then rotates', nativeBuild, t => {
  const { root, m, exists, binary, run } = buildFixture(t);
  fs.writeFileSync(path.join(root, 'scripts/verify.sh'), 'exit 5\n');
  const verifyFailure = run();
  assert.equal(verifyFailure.status, 5, verifyFailure.stderr + verifyFailure.stdout);
  assert.ok(!exists(binary) && !exists('bin/.build-lock'));
  assert.equal(m.read().builds.length, 2);
  fs.writeFileSync(path.join(root, 'scripts/verify.sh'), 'exit 0\n');
  fs.writeFileSync(path.join(root, 'fail-compiler'), 'yes');
  const compilerFailure = run();
  assert.equal(compilerFailure.status, 7, compilerFailure.stderr + compilerFailure.stdout);
  assert.ok(!exists(binary) && exists('bin/leaguepingo-v1.exe'));
  fs.unlinkSync(path.join(root, 'fail-compiler'));
  const success = run();
  assert.equal(success.status, 0, success.stderr + success.stdout);
  assert.ok(exists(binary) && exists('bin/leaguepingo-v2.exe') && !exists('bin/leaguepingo-v1.exe'));
  assert.equal(m.read().builds.at(-1).accepted, false);
  assert.notEqual(run().status, 0); // Existing output is never overwritten.
});

test('build shell: deleted old files do not block verification, recovery from failure or a new build', nativeBuild, t => {
  const { root, m, exists, binary, run } = buildFixture(t);
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v1.exe'));
  fs.unlinkSync(path.join(root, 'bin/ui-v2.png'));
  fs.writeFileSync(path.join(root, 'scripts/verify.sh'), 'exit 5\n');
  const failure = run();
  assert.equal(failure.status, 5, failure.stderr + failure.stdout);
  assert.ok(!exists(binary) && !exists('bin/.build-lock'));
  assert.ok(exists('bin/leaguepingo-v2.exe') && !exists('bin/ui-v1.png'));
  assert.deepEqual(m.read().builds.map(b => [b.file, b.evidence]), [['bin/leaguepingo-v2.exe', []]]);
  fs.writeFileSync(path.join(root, 'scripts/verify.sh'), 'exit 0\n');
  fs.writeFileSync(path.join(root, 'fail-compiler'), 'yes');
  const compilerFailure = run();
  assert.equal(compilerFailure.status, 7, compilerFailure.stderr + compilerFailure.stdout);
  assert.ok(!exists(binary) && exists('bin/leaguepingo-v2.exe'));
  fs.unlinkSync(path.join(root, 'fail-compiler'));
  const success = run();
  assert.equal(success.status, 0, success.stderr + success.stdout);
  assert.ok(exists(binary) && exists('bin/leaguepingo-v2.exe'));
  assert.equal(m.audit().current.accepted, false);
  fs.unlinkSync(path.join(root, 'bin/leaguepingo-v2.exe'));
  fs.unlinkSync(path.join(root, binary));
  const allDeleted = run('bin/leaguepingo-v4.exe');
  assert.equal(allDeleted.status, 0, allDeleted.stderr + allDeleted.stdout);
  assert.deepEqual(m.audit().retained, ['bin/leaguepingo-v4.exe']);
});
