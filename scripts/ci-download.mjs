import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { manager } from './artifacts.mjs';
import { parseVersion } from './version.mjs';

const digest = file => createHash('sha256').update(fs.readFileSync(file)).digest('hex');

// Export only a portable build receipt. The runner's retention/acceptance
// manifest belongs to that machine and must never accompany a download.
export function prepareDownload(root, { commit, repository, runID, attempt }) {
  if (!/^[a-f0-9]{40}$/.test(commit ?? '') || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository ?? '') ||
      !/^[1-9]\d*$/.test(runID ?? '') || !/^[1-9]\d*$/.test(attempt ?? '')) throw new Error('Invalid CI build identity');
  const current = manager(root).audit().current;
  if (current.file !== 'bin/leaguepingo-ci.exe' || !/^[a-f0-9]{7,40}$/.test(current.commit) ||
      !commit.startsWith(current.commit)) throw new Error('Download must match the clean current CI commit');
  const version = parseVersion(fs.readFileSync(path.join(root, 'VERSION'), 'utf8')).value;
  const installer = path.join(root, 'artifacts/installer-check/leaguepingo-ci-installer.exe');
  if (!fs.lstatSync(installer).isFile()) throw new Error('Verified installer file is missing');
  const folder = path.join(root, 'artifacts/ci-download');
  fs.mkdirSync(folder); // A rerun has a fresh checkout; never overwrite an existing bundle.
  const pairs = [[path.join(root, current.file), 'leaguepingo.exe'], [installer, 'leaguepingo-setup.exe']];
  const files = pairs.map(([source, name]) => {
    const target = path.join(folder, name);
    fs.copyFileSync(source, target, fs.constants.COPYFILE_EXCL);
    return { name, sha256: digest(target), bytes: fs.statSync(target).size };
  });
  if (files[0].sha256 !== current.sha256) throw new Error('Downloaded executable differs from audited build');
  const runURL = `https://github.com/${repository}/actions/runs/${runID}`;
  const receipt = { schemaVersion: 1, version, commit, platform: 'windows-amd64', runURL,
    runAttempt: Number(attempt), packagedAt: new Date().toISOString(), realClientAcceptance: 'pending', files };
  fs.writeFileSync(path.join(folder, 'BUILD-INFO.json'), JSON.stringify(receipt, null, 2) + '\n');
  fs.writeFileSync(path.join(folder, 'README.txt'), `LeaguePingo 开发测试版 ${version} (${commit.slice(0, 8)})

1. 将整个下载 ZIP 解压到一个新的版本目录。
2. 从托盘退出正在运行的旧版助手，再双击 leaguepingo.exe，即可开始测试。
3. 如需开始菜单/桌面快捷方式，可改用 leaguepingo-setup.exe 安装；两种方式任选一种。

无需 Go、Node 或 Wails。Windows 需要 WebView2 Runtime；缺失时按程序提示安装。
现有本机偏好沿用。请保留最近可用版本；本下载不会自动清理旧版或登记游戏验收。
这是开发测试产物，尚未配置发布签名；下载后仍需在游戏电脑验证实际功能。

提交：${commit}
构建记录：${runURL}
反馈问题时附上本提交号。BUILD-INFO.json 与 SHA256SUMS.txt 用于核对文件。
可在 PowerShell 执行 Get-FileHash .\\leaguepingo.exe -Algorithm SHA256 比对 SHA256SUMS.txt。
`);
  const hashes = [...files.map(f => `${f.sha256}  ${f.name}`),
    ...['BUILD-INFO.json', 'README.txt'].map(name => `${digest(path.join(folder, name))}  ${name}`)];
  fs.writeFileSync(path.join(folder, 'SHA256SUMS.txt'), hashes.join('\n') + '\n');
  return { name: `leaguepingo-dev-windows-amd64-${commit.slice(0, 8)}-${runID}-${attempt}`, folder, runURL };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.platform !== 'win32' || process.env.GITHUB_ACTIONS !== 'true') throw new Error('Download packaging requires Windows GitHub Actions');
    const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
    const result = prepareDownload(root, { commit: process.env.GITHUB_SHA, repository: process.env.GITHUB_REPOSITORY,
      runID: process.env.GITHUB_RUN_ID, attempt: process.env.GITHUB_RUN_ATTEMPT });
    fs.appendFileSync(process.env.GITHUB_OUTPUT, `artifact_name=${result.name}\n`);
    console.log(`Prepared ${result.name}`);
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
