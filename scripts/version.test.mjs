import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkVersion, parseVersion } from './version.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const files = Object.fromEntries(['VERSION', 'preferences.go', 'build/config.yml', 'build/windows/info.json',
  'build/windows/wails.exe.manifest', 'build/windows/nsis/wails_tools.nsh', 'update.json']
  .map(file => [file, fs.readFileSync(path.join(root, file), 'utf8')]));

test('tracked application and installer versions share VERSION', () => {
  assert.equal(checkVersion(file => files[file]).value, '0.2.0-dev');
});

test('semantic version source yields Windows numeric version', () => {
  assert.deepEqual(parseVersion('1.2.3-rc.1\n'), { value: '1.2.3-rc.1', base: '1.2.3', numeric: '1.2.3.0' });
  assert.throws(() => parseVersion('1.2.3-rc.01'), /leading zero/);
  for (const value of ['1.2', '01.2.3', '1.2.3-', 'v1.2.3']) assert.throws(() => parseVersion(value));
});

test('version drift and unsafe release URL are rejected', () => {
  assert.throws(() => checkVersion(file => file === 'preferences.go' ? files[file].replace('0.2.0-dev', '9.9.9') : files[file]), /mismatch/);
  assert.throws(() => checkVersion(file => file === 'update.json' ? files[file].replace('github.com/ZJing1125', 'evil.example') : files[file]), /manifest/);
});
