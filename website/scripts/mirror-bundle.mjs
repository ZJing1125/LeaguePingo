import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export function localDownloads(data) {
  const result = structuredClone(data);
  for (const key of ['stable', 'preview']) {
    const release = result[key];
    if (!release) continue;
    if (!/^[a-zA-Z0-9][\w.+-]{0,120}$/.test(release.tag)) throw new Error('Invalid mirror release tag');
    for (const [name, file] of Object.entries(release.files)) {
      if (!/^[\w.-]+$/.test(name) || name === '.' || name === '..') throw new Error('Invalid mirror filename');
      file.url = `/releases/${release.tag}/${name}`;
    }
  }
  return result;
}

function run(command, args, options) {
  const result = spawnSync(command, args, { stdio: 'inherit', ...options });
  if (result.status !== 0) throw new Error(`Mirror build failed: ${command}`);
}

export function buildMirror(root, { browser = false } = {}) {
  const commit = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' });
  if (commit.status !== 0 || !/^[a-f0-9]{40}$/.test(commit.stdout.trim())) throw new Error('Unknown website commit');
  // Astro moves prerendered assets with rename(), so output must share its filesystem.
  fs.mkdirSync(path.join(root, '.astro'), { recursive: true });
  const directory = fs.mkdtempSync(path.join(root, '.astro/mirror-build-'));
  const output = path.join(root, 'dist');
  const archive = path.join(output, 'mirror.tar.gz');
  try {
    const env = { ...process.env, WEBSITE_MIRROR: '1', WEBSITE_BASE: '/', WEBSITE_OUT_DIR: directory,
      WEBSITE_CANONICAL_BASE: process.env.WEBSITE_BASE || '/LeaguePingo/' };
    const astro = JSON.parse(fs.readFileSync(path.join(root, 'node_modules/astro/package.json'), 'utf8'));
    run(process.execPath, [path.join(root, 'node_modules/astro', astro.bin.astro), 'build'], { cwd: root, env });
    const releases = JSON.parse(fs.readFileSync(path.join(root, 'src/data/releases.json'), 'utf8'));
    const built = JSON.parse(fs.readFileSync(path.join(directory, 'downloads.json'), 'utf8'));
    if (JSON.stringify(built) !== JSON.stringify(localDownloads(releases))) throw new Error('Mirror download manifest differs');
    if (browser) run(process.execPath, ['tests/browser.mjs'], { cwd: root,
      env: { ...env, WEBSITE_DIST: directory } });
    fs.mkdirSync(output, { recursive: true });
    run('tar', ['-czf', archive + '.tmp', '-C', directory, '.']);
    const bytes = fs.readFileSync(archive + '.tmp');
    const descriptor = { schemaVersion: 1, websiteCommit: commit.stdout.trim(),
      archive: { name: 'mirror.tar.gz', size: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }, releases };
    fs.renameSync(archive + '.tmp', archive);
    fs.writeFileSync(path.join(output, 'mirror.json.tmp'), JSON.stringify(descriptor, null, 2) + '\n');
    fs.renameSync(path.join(output, 'mirror.json.tmp'), path.join(output, 'mirror.json'));
    console.log('Verified mirror bundle:', descriptor.websiteCommit);
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
    fs.rmSync(archive + '.tmp', { force: true });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { buildMirror(path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..'), { browser: process.argv.includes('--browser') }); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
