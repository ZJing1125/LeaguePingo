import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { parseVersion } from './version.mjs';

const names = ['BUILD-INFO.json', 'README.txt', 'SHA256SUMS.txt', 'leaguepingo-setup.exe', 'leaguepingo.exe'];
const hash = data => createHash('sha256').update(data).digest('hex');

export function inspectBundle(folder, { repository, commit, runID, version }) {
  if (!/^[\w.-]+\/[\w.-]+$/.test(repository ?? '') || !/^[a-f0-9]{40}$/.test(commit ?? '') ||
      !/^[1-9]\d*$/.test(runID ?? '')) throw new Error('Invalid release identity');
  parseVersion(version);
  if (JSON.stringify(fs.readdirSync(folder).sort()) !== JSON.stringify(names)) throw new Error('Unexpected release files');
  const files = names.map(name => {
    const file = path.join(folder, name), stat = fs.lstatSync(file);
    if (!stat.isFile() || stat.isSymbolicLink()) throw new Error('Release assets must be regular files');
    return { name, file, size: stat.size, digest: 'sha256:' + hash(fs.readFileSync(file)) };
  });
  const receipt = JSON.parse(fs.readFileSync(path.join(folder, 'BUILD-INFO.json'), 'utf8'));
  const runURL = `https://github.com/${repository}/actions/runs/${runID}`;
  if (receipt.schemaVersion !== 1 || receipt.commit !== commit || receipt.version !== version ||
      receipt.platform !== 'windows-amd64' || receipt.runURL !== runURL || receipt.realClientAcceptance !== 'pending') {
    throw new Error('Release receipt does not match the verified build');
  }
  const checksums = fs.readFileSync(path.join(folder, 'SHA256SUMS.txt'), 'utf8').trim().split('\n');
  const expected = files.filter(f => f.name !== 'SHA256SUMS.txt').map(f => `${f.digest.slice(7)}  ${f.name}`).sort();
  if (JSON.stringify(checksums.sort()) !== JSON.stringify(expected)) throw new Error('Release checksum mismatch');
  const binaries = files.filter(f => f.name.endsWith('.exe'));
  if (!Array.isArray(receipt.files) || receipt.files.length !== 2 || binaries.some(file =>
    !receipt.files.some(item => item.name === file.name && item.bytes === file.size && 'sha256:' + item.sha256 === file.digest))) {
    throw new Error('Release binary receipt mismatch');
  }
  return { files, repository, commit, runURL, tag: `v${version}.g${commit.slice(0, 8)}`, version };
}

export function verifyAssets(release, bundle, complete = true) {
  if (release.tag_name !== bundle.tag || release.target_commitish !== bundle.commit || release.prerelease !== true) {
    throw new Error('Existing release identity does not match');
  }
  const seen = new Set();
  for (const asset of release.assets) {
    const expected = bundle.files.find(file => file.name === asset.name);
    if (!expected || seen.has(asset.name) || asset.state !== 'uploaded' || asset.size !== expected.size || asset.digest !== expected.digest) {
      throw new Error('Existing release asset differs; refusing to overwrite');
    }
    seen.add(asset.name);
  }
  if (complete && seen.size !== bundle.files.length) throw new Error('Release assets are incomplete');
}

function gh(args, optional = false) {
  const result = spawnSync('gh', args, { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
  if (result.status !== 0) {
    if (optional && /HTTP 404\b/.test(result.stderr || '')) return null;
    throw new Error(`GitHub ${args[0]} operation failed (exit ${result.status}); release remains unpublished if still a draft`);
  }
  return result.stdout;
}

export function publish(bundle, execute = gh) {
  const api = `repos/${bundle.repository}`;
  const json = (args, optional = false) => {
    const value = execute(['api', ...args], optional);
    return value === null ? null : JSON.parse(value);
  };
  let ref = json([`${api}/git/ref/tags/${bundle.tag}`], true);
  if (!ref) ref = json(['--method', 'POST', `${api}/git/refs`, '-f', `ref=refs/tags/${bundle.tag}`, '-f', `sha=${bundle.commit}`]);
  if (ref.object?.type !== 'commit' || ref.object.sha !== bundle.commit) throw new Error('Release tag points to a different commit');
  let release = json([`${api}/releases/tags/${bundle.tag}`], true);
  if (!release) {
    const notes = `Windows amd64 开发测试版，由 GitHub Actions 自动构建。\n\n` +
      `### 下载\n\n下载 **leaguepingo.exe** 可直接运行；需要安装时选择 **leaguepingo-setup.exe**。从托盘退出旧版后启动新版。\n\n` +
      `无需 Go、Node 或 Wails；Windows 需要 WebView2 Runtime，缺失时按程序提示安装。现有本机偏好沿用。\n\n` +
      `提交：\`${bundle.commit}\`；程序版本：\`${bundle.version}\`。\n\n` +
      `[本次构建及变更](${bundle.runURL})已通过 Linux 检查、质量检查、Windows 完整验证与构建、安装器隔离安装/卸载。\n\n` +
      `BUILD-INFO.json 记录提交与文件信息，SHA256SUMS.txt 提供校验值，README.txt 提供离线说明。\n\n` +
      `本版为预发布测试版，尚未配置发布签名；国服数据及实际游戏体验仍需本机验收。保留上一版可用程序，反馈时附提交号。`;
    release = json(['--method', 'POST', `${api}/releases`, '-f', `tag_name=${bundle.tag}`,
      '-f', `target_commitish=${bundle.commit}`, '-f', `name=LeaguePingo ${bundle.version} · ${bundle.commit.slice(0, 8)}`,
      '-F', 'draft=true', '-F', 'prerelease=true', '-f', 'make_latest=false', '-f', `body=${notes}`]);
  }
  verifyAssets(release, bundle, !release.draft);
  if (!release.draft) return release.html_url;
  const missing = bundle.files.filter(file => !release.assets.some(asset => asset.name === file.name));
  if (missing.length) execute(['release', 'upload', bundle.tag, ...missing.map(file => file.file), '--repo', bundle.repository]);
  release = json([`${api}/releases/${release.id}`]);
  verifyAssets(release, bundle);
  const published = json(['--method', 'PATCH', `${api}/releases/${release.id}`, '-F', 'draft=false', '-F', 'prerelease=true', '-f', 'make_latest=false']);
  if (published.draft || !published.prerelease) throw new Error('Release publication was not confirmed');
  verifyAssets(published, bundle);
  return published.html_url;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.env.GITHUB_ACTIONS !== 'true' || process.env.GITHUB_REF !== 'refs/heads/dev') throw new Error('Release publishing requires dev GitHub Actions');
    const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
    const bundle = inspectBundle(path.join(root, 'artifacts/release'), { repository: process.env.GITHUB_REPOSITORY,
      commit: process.env.GITHUB_SHA, runID: process.env.GITHUB_RUN_ID, version: fs.readFileSync(path.join(root, 'VERSION'), 'utf8').trim() });
    const url = publish(bundle);
    fs.appendFileSync(process.env.GITHUB_STEP_SUMMARY, `### Releases 下载\n\n[${bundle.tag}](${url})\n\n程序和安装器已上传并核对 SHA-256。\n`);
    console.log(`Published ${url}`);
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
