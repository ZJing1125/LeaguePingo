import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { localDownloads } from '../scripts/mirror-bundle.mjs';

test('mirror serves every asset locally while preserving GitHub provenance and original manifest', () => {
  const input = JSON.parse(fs.readFileSync(new URL('../src/data/releases.json', import.meta.url), 'utf8'));
  const original = structuredClone(input);
  const local = localDownloads(input);
  for (const key of ['stable', 'preview']) {
    if (!local[key]) continue;
    assert.equal(local[key].url, input[key].url);
    for (const [name, file] of Object.entries(local[key].files)) {
      assert.equal(file.url, `/releases/${local[key].tag}/${name}`);
      assert.equal(file.sha256, input[key].files[name].sha256);
      assert.equal(file.size, input[key].files[name].size);
    }
  }
  assert.deepEqual(input, original);
});

test('mirror refuses traversal in release tags or filenames', () => {
  for (const data of [{preview:{tag:'../escape',files:{}}}, {preview:{tag:'v1.0.0',files:{'../escape':{}}}}]) {
    assert.throws(() => localDownloads(data), /Invalid mirror/);
  }
});

test('Pages publication requires server updater checks and a verified root-path browser build', () => {
  const workflow = fs.readFileSync(new URL('../../.github/workflows/website.yml', import.meta.url), 'utf8');
  assert.match(workflow, /python3 -m unittest discover -s website\/deploy -p 'test_\*\.py'/);
  assert.match(workflow, /bash -n website\/deploy\/install\.sh/);
  assert.match(workflow, /run: BROWSER_EXECUTABLE=.*npm --prefix website run build:mirror -- --browser/);
  assert(workflow.indexOf('run build:mirror') < workflow.indexOf('- name: Upload website'));
  assert.doesNotMatch(workflow, /continue-on-error:|contents: write|secrets: inherit/);
});
