import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { createHash } from 'node:crypto';
import { gh, inspectBundle, publish, verifyAssets } from './publish-release.mjs';

const identity = { repository: 'example/project', commit: 'a'.repeat(40), runID: '123', version: '0.2.0-dev' };
test('CLI HTTP status uses structured responses and logs only method and endpoint', () => {
  const args = ['api', '--method', 'POST', 'repos/example/project/releases', '-f', 'body=private notes'];
  assert.equal(gh(args, true, () => ({status: 1, stdout: '{"status":"404"}', stderr: 'Not Found'})), null);
  assert.equal(gh(args, true, () => ({status: 1, stdout: '', stderr: 'gh: Not Found (HTTP 404)'})), null);
  assert.throws(() => gh(args, true, () => ({status: 1, stdout: '{"status":403}', stderr: 'private notes'})), error => {
    assert.match(error.message, /POST repos\/example\/project\/releases failed \(HTTP 403/);
    assert(!error.message.includes('private notes'));
    return true;
  });
});
const hash = data => createHash('sha256').update(data).digest('hex');
function fixture(t) {
  const folder = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-release-'));
  t.after(() => fs.rmSync(folder, { recursive: true, force: true }));
  const files = ['leaguepingo.exe', 'leaguepingo-setup.exe'].map(name => {
    const data = Buffer.from('MZ synthetic ' + name);
    fs.writeFileSync(path.join(folder, name), data);
    return { name, sha256: hash(data), bytes: data.length };
  });
  fs.writeFileSync(path.join(folder, 'README.txt'), 'synthetic instructions');
  fs.writeFileSync(path.join(folder, 'BUILD-INFO.json'), JSON.stringify({ schemaVersion: 1, ...identity,
    platform: 'windows-amd64', runURL: 'https://github.com/example/project/actions/runs/123', realClientAcceptance: 'pending', files }));
  fs.writeFileSync(path.join(folder, 'SHA256SUMS.txt'), [...files.map(f => f.name), 'README.txt', 'BUILD-INFO.json']
    .map(name => `${hash(fs.readFileSync(path.join(folder, name)))}  ${name}`).join('\n') + '\n');
  return { folder, bundle: inspectBundle(folder, identity) };
}
function remote(bundle, assets = [], draft = true) {
  return { id: 1, tag_name: bundle.tag, target_commitish: bundle.commit, prerelease: true, draft,
    html_url: `https://github.com/${bundle.repository}/releases/tag/${bundle.tag}`, assets };
}
const assets = bundle => bundle.files.map(({ name, size, digest }) => ({ name, size, digest, state: 'uploaded' }));

test('release refuses unrelated builds, changed files and unexpected files before publication', t => {
  const { folder } = fixture(t);
  assert.throws(() => inspectBundle(folder, { ...identity, commit: 'b'.repeat(40) }), /receipt/);
  assert.throws(() => inspectBundle(folder, { ...identity, runID: '456' }), /receipt/);
  fs.appendFileSync(path.join(folder, 'leaguepingo.exe'), 'changed');
  assert.throws(() => inspectBundle(folder, identity), /checksum/);
  fs.writeFileSync(path.join(folder, 'artifacts.json'), '{}');
  assert.throws(() => inspectBundle(folder, identity), /Unexpected release files/);
});
test('publication uploads to a draft, verifies all hashes, then publishes only a prerelease', t => {
  const { bundle } = fixture(t), calls = [];
  let release = null, tagged = false;
  const execute = args => {
    calls.push(args);
    if (args[0] === 'release') {
      assert.equal(release.draft, true);
      assert.equal(args.filter(arg => arg.endsWith('.exe')).length, 2);
      release.assets = assets(bundle);
      return '';
    }
    if (args.some(arg => arg.endsWith('/git/refs'))) {
      tagged = true;
      return JSON.stringify({ object: { type: 'commit', sha: bundle.commit } });
    }
    if (args.some(arg => arg.includes('/git/ref/tags/'))) return tagged ? JSON.stringify({ object: { type: 'commit', sha: bundle.commit } }) : null;
    if (args.includes('POST')) {
      assert(args.includes('draft=true') && args.includes('prerelease=true') && args.includes('make_latest=false'));
      release = remote(bundle);
    } else if (args.includes('PATCH')) {
      verifyAssets(release, bundle);
      assert(args.includes('draft=false') && args.includes('prerelease=true') && args.includes('make_latest=false'));
      release.draft = false;
    }
    return release ? JSON.stringify(release) : null;
  };
  assert.equal(publish(bundle, execute), remote(bundle).html_url);
  const writes = calls.filter(c => c.includes('POST') || c.includes('PATCH') || c[0] === 'release').length;
  assert.equal(publish(bundle, execute), remote(bundle).html_url);
  assert.equal(calls.filter(c => c.includes('POST') || c.includes('PATCH') || c[0] === 'release').length, writes, 'published rerun is read-only');
});
test('mismatched tag, partial published assets and changed remote assets cannot be replaced', t => {
  const { bundle } = fixture(t);
  assert.throws(() => publish(bundle, () => JSON.stringify({ object: { type: 'commit', sha: 'b'.repeat(40) } })), /different commit/);
  assert.throws(() => verifyAssets(remote(bundle, assets(bundle).slice(1), false), bundle), /incomplete/);
  const wrong = assets(bundle); wrong[0].digest = 'sha256:' + '0'.repeat(64);
  assert.throws(() => verifyAssets(remote(bundle, wrong), bundle), /refusing to overwrite/);
  verifyAssets(remote(bundle, assets(bundle).slice(1)), bundle, false);
});
test('failed uploads leave the draft unpublished for a safe rerun', t => {
  const { bundle } = fixture(t), calls = [];
  const execute = args => {
    calls.push(args);
    if (args[0] === 'release') throw new Error('synthetic upload failure');
    if (args.some(arg => arg.includes('/git/ref/'))) return JSON.stringify({ object: { type: 'commit', sha: bundle.commit } });
    return JSON.stringify(remote(bundle, assets(bundle).slice(0, 1)));
  };
  assert.throws(() => publish(bundle, execute), /upload failure/);
  assert(!calls.some(args => args.includes('PATCH')));
  assert.equal(calls.at(-1).filter(arg => bundle.files.some(f => f.file === arg)).length, 4, 'only missing draft assets are uploaded');
});
