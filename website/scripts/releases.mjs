import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';

export const repository = 'ZJing1125/LeaguePingo';
const origin = `https://github.com/${repository}`;
const names = ['BUILD-INFO.json', 'SHA256SUMS.txt', 'README.txt', 'leaguepingo.exe', 'leaguepingo-setup.exe'];
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
export function selectChannels(releases) {
  const published = releases.filter(r => !r.draft && Number.isFinite(Date.parse(r.published_at)))
    .sort((a, b) => Date.parse(b.published_at) - Date.parse(a.published_at));
  return { stable: published.find(r => !r.prerelease) ?? null, preview: published.find(r => r.prerelease) ?? null };
}
export function manifest(release, content) {
  if (!release || release.draft || !/^[\w.+-]+$/.test(release.tag_name)) throw new Error('Invalid published release');
  const url = `${origin}/releases/tag/${release.tag_name}`;
  if (release.html_url !== url) throw new Error('Unexpected release origin');
  const files = Object.fromEntries(names.map(name => {
    const found = release.assets.filter(a => a.name === name);
    if (found.length !== 1) throw new Error(`Missing or duplicate release asset: ${name}`);
    const a = found[0];
    if (a.state !== 'uploaded' || !(a.size > 0) || !/^sha256:[a-f0-9]{64}$/.test(a.digest) ||
        a.browser_download_url !== `${origin}/releases/download/${release.tag_name}/${name}`) throw new Error('Invalid release asset');
    return [name, { url: a.browser_download_url, size: a.size, sha256: a.digest.slice(7) }];
  }));
  for (const name of ['BUILD-INFO.json', 'SHA256SUMS.txt', 'README.txt']) {
    if (!content[name] || content[name].length !== files[name].size || digest(content[name]) !== files[name].sha256) throw new Error('Metadata checksum mismatch');
  }
  const info = JSON.parse(content['BUILD-INFO.json']);
  if (info.schemaVersion !== 1 || info.platform !== 'windows-amd64' || !/^[a-f0-9]{40}$/.test(info.commit) ||
      release.target_commitish !== info.commit || !/^\d+\.\d+\.\d+(?:-[\w.-]+)?$/.test(info.version) ||
      !info.runURL?.startsWith(`${origin}/actions/runs/`) || !/^\d+$/.test(info.runURL.split('/').at(-1))) throw new Error('Build identity mismatch');
  if (release.tag_name !== `v${info.version}.g${info.commit.slice(0, 8)}` && release.tag_name !== `v${info.version}`) throw new Error('Version tag mismatch');
  const sums = content['SHA256SUMS.txt'].toString().trim().split('\n');
  const expected = names.filter(n => n !== 'SHA256SUMS.txt').map(n => `${files[n].sha256}  ${n}`);
  if (JSON.stringify(sums.sort()) !== JSON.stringify(expected.sort())) throw new Error('Release checksums differ');
  if (info.files?.length !== 2 || ['leaguepingo.exe', 'leaguepingo-setup.exe'].some(name =>
    !info.files.some(f => f.name === name && f.bytes === files[name].size && f.sha256 === files[name].sha256))) throw new Error('Binary identity mismatch');
  return { version: info.version, tag: release.tag_name, commit: info.commit, publishedAt: release.published_at,
    channel: release.prerelease ? 'preview' : 'stable', url, buildURL: info.runURL, files };
}
function gh(args) {
  const result = spawnSync('gh', args, { encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
  if (result.status !== 0) throw new Error('Release retrieval failed; existing download manifest preserved');
  return result.stdout;
}
export function syncRelease(root) {
  const releases = [];
  for (let page = 1; page <= 10; page++) {
    const batch = JSON.parse(gh(['api', `repos/${repository}/releases?per_page=100&page=${page}`]));
    releases.push(...batch);
    if (batch.length < 100 || (releases.some(r => !r.draft && !r.prerelease) && releases.some(r => !r.draft && r.prerelease))) break;
  }
  const selected = selectChannels(releases), channels = {};
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-website-'));
  try {
    for (const [key, release] of Object.entries(selected)) {
      if (!release) { channels[key] = null; continue; }
      const folder = path.join(temp, key);
      gh(['release', 'download', release.tag_name, '--repo', repository, '--dir', folder,
        '--pattern', 'BUILD-INFO.json', '--pattern', 'SHA256SUMS.txt', '--pattern', 'README.txt']);
      channels[key] = manifest(release, Object.fromEntries(['BUILD-INFO.json', 'SHA256SUMS.txt', 'README.txt'].map(name => [name, fs.readFileSync(path.join(folder, name))])));
    }
    if (!channels.stable && !channels.preview) throw new Error('No verified release available');
    const data = JSON.stringify({ schemaVersion: 1, ...channels }, null, 2) + '\n';
    const target = path.join(root, 'src/data/releases.json');
    fs.writeFileSync(target + '.tmp', data);
    fs.renameSync(target + '.tmp', target);
    console.log('Verified release manifest:', (channels.stable ?? channels.preview).tag);
  } finally { fs.rmSync(temp, { recursive: true, force: true }); }
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { syncRelease(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
