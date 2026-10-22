import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { root, checkIcons, iconFrames, iconSizes, pngSize } from './app-icons.mjs';

test('approved C artwork has a complete identical desktop/UI bundle for native and high-DPI sizes', () => {
  const before = fs.readFileSync(path.join(root, 'build/brand.json'));
  assert.deepEqual(checkIcons(), { design: 'C-satin-enamel-scout', sizes: iconSizes, files: 12 });
  assert.deepEqual(fs.readFileSync(path.join(root, 'build/brand.json')), before, 'validation must be read-only');
  assert.deepEqual(pngSize(fs.readFileSync(path.join(root, 'build/appicon.png'))), { width: 1254, height: 1254 });
});

test('missing or stale UI assets fail validation rather than silently diverging from the executable', t => {
  const folder = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-brand-'));
  t.after(() => fs.rmSync(folder, { recursive: true, force: true }));
  const manifest = JSON.parse(fs.readFileSync(path.join(root, 'build/brand.json')));
  for (const file of ['build/brand.json', ...Object.keys(manifest.files)]) {
    fs.mkdirSync(path.dirname(path.join(folder, file)), { recursive: true });
    fs.copyFileSync(path.join(root, file), path.join(folder, file));
  }
  checkIcons(folder);
  const file = path.join(folder, 'frontend/public/brand/icon-32.png');
  fs.appendFileSync(file, 'changed');
  assert.throws(() => checkIcons(folder), /Changed brand asset/);
  fs.unlinkSync(file);
  assert.throws(() => checkIcons(folder), /ENOENT/);
});

test('an incomplete ICO or a corrupt payload cannot be published as the approved icon bundle', () => {
  const original = fs.readFileSync(path.join(root, 'build/windows/icon.ico'));
  assert.throws(() => iconFrames(original.subarray(0, original.length - 1)), /range/);
  const invalid = Buffer.from(original);
  invalid.writeUInt32LE(0, 18);
  assert.throws(() => iconFrames(invalid), /range/);
  const frames = iconFrames(original);
  assert.deepEqual([...frames.keys()].sort((a,b) => a-b), iconSizes);
  for (const size of iconSizes) assert.deepEqual(pngSize(frames.get(size)), { width: size, height: size });
});
