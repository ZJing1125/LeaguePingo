// Explicit asset maintenance only. Normal builds validate committed assets.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const iconSizes = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256];
const sourceFile = 'build/appicon.png';
const icoFile = 'build/windows/icon.ico';
const manifestFile = 'build/brand.json';
const pngSignature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
const hash = data => createHash('sha256').update(data).digest('hex');

export function pngSize(data) {
  if (data.length < 33 || !data.subarray(0, 8).equals(pngSignature) || data.toString('ascii', 12, 16) !== 'IHDR') throw new Error('Invalid icon PNG');
  return { width: data.readUInt32BE(16), height: data.readUInt32BE(20) };
}

// Wails beta.29 writes lossless PNG payloads into its Windows ICO. Reuse those
// exact pixels in the web UI, rather than resampling the artwork a second time.
export function iconFrames(data) {
  if (data.length < 6 || data.readUInt16LE(0) !== 0 || data.readUInt16LE(2) !== 1) throw new Error('Invalid Windows ICO');
  const count = data.readUInt16LE(4);
  const tableEnd = 6 + count * 16;
  if (count !== iconSizes.length || data.length < tableEnd) throw new Error('Incomplete icon size table');
  const frames = new Map();
  let nextOffset = tableEnd;
  for (let index = 0; index < count; index++) {
    const at = 6 + index * 16;
    const width = data[at] || 256, height = data[at + 1] || 256;
    const length = data.readUInt32LE(at + 8), offset = data.readUInt32LE(at + 12);
    if (offset !== nextOffset || length < 33 || offset + length > data.length) throw new Error('Invalid icon image range');
    const png = data.subarray(offset, offset + length);
    const size = pngSize(png);
    if (width !== height || size.width !== width || size.height !== height || !iconSizes.includes(width) || frames.has(width)) throw new Error('Invalid icon dimensions');
    frames.set(width, png);
    nextOffset = offset + length;
  }
  if (nextOffset !== data.length) throw new Error('Unexpected data after icon images');
  return frames;
}

export function checkIcons(folder = root) {
  const read = file => fs.readFileSync(path.join(folder, file));
  const manifest = JSON.parse(read(manifestFile));
  if (manifest.design !== 'C-satin-enamel-scout' || JSON.stringify(manifest.sizes) !== JSON.stringify(iconSizes)) throw new Error('Unrecognised brand manifest');
  const expectedFiles = [sourceFile, icoFile, ...iconSizes.map(size => `frontend/public/brand/icon-${size}.png`)];
  if (JSON.stringify(Object.keys(manifest.files).sort()) !== JSON.stringify(expectedFiles.sort())) throw new Error('Incomplete brand asset manifest');
  for (const [file, info] of Object.entries(manifest.files)) {
    const data = read(file);
    if (info.bytes !== data.length || info.sha256 !== hash(data)) throw new Error(`Changed brand asset: ${file}; regenerate explicitly`);
  }
  const source = pngSize(read(sourceFile));
  if (source.width !== source.height || source.width < 1024) throw new Error('Brand source must be a high-resolution square PNG');
  const frames = iconFrames(read(icoFile));
  for (const size of iconSizes) {
    if (!frames.get(size).equals(read(`frontend/public/brand/icon-${size}.png`))) throw new Error(`Desktop/UI icon mismatch at ${size}px`);
  }
  return { design: manifest.design, sizes: iconSizes, files: expectedFiles.length };
}

function generate() {
  const size = pngSize(fs.readFileSync(path.join(root, sourceFile)));
  if (size.width !== size.height || size.width < 1024) throw new Error('Use the approved high-resolution square PNG');
  const result = spawnSync('wails3', ['generate', 'icons', '-input', sourceFile,
    '-sizes', [...iconSizes].reverse().join(','), '-windowsfilename', icoFile, '-macfilename', ''],
  { cwd: root, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Wails icon generation failed (${result.status})`);
  const frames = iconFrames(fs.readFileSync(path.join(root, icoFile)));
  fs.mkdirSync(path.join(root, 'frontend/public/brand'), { recursive: true });
  for (const [size, data] of frames) fs.writeFileSync(path.join(root, `frontend/public/brand/icon-${size}.png`), data);
  const files = {};
  for (const file of [sourceFile, icoFile, ...iconSizes.map(s => `frontend/public/brand/icon-${s}.png`)]) {
    const data = fs.readFileSync(path.join(root, file));
    files[file] = { bytes: data.length, sha256: hash(data) };
  }
  fs.writeFileSync(path.join(root, manifestFile), JSON.stringify({ design: 'C-satin-enamel-scout', sizes: iconSizes, files }, null, 2) + '\n');
  return checkIcons();
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const command = process.argv[2] ?? 'check';
    if (!['check', 'generate'].includes(command)) throw new Error('Usage: node scripts/app-icons.mjs [check|generate]');
    console.log(JSON.stringify(command === 'generate' ? generate() : checkIcons()));
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
