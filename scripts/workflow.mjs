import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { manager, runningExecutables } from './artifacts.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const policyFiles = ['AGENTS.md', 'Taskfile.yml', 'build/windows/Taskfile.yml',
  'build/Taskfile.yml', 'build/config.yml', 'scripts/build.sh', 'scripts/run.sh',
  'scripts/dev.sh', 'scripts/verify.sh', 'docs/ci/verify.yml.example'];

// A deliberately restricted policy check, not a general YAML parser. Keep
// entry tasks simple and also test their expansion with the real Wails runner.
function taskBlocks(text) {
  const tasks = text.split(/^tasks:\s*$/m)[1];
  if (!tasks) throw new Error('Missing tasks section');
  const result = new Map();
  const matches = [...tasks.matchAll(/^  ([\w:]+):\s*$/gm)];
  matches.forEach((m, i) => {
    if (result.has(m[1])) throw new Error(`Duplicate task: ${m[1]}`);
    result.set(m[1], tasks.slice(m.index + m[0].length, matches[i + 1]?.index ?? tasks.length));
  });
  return result;
}

export function checkWorkflow(read) {
  const contents = Object.fromEntries(policyFiles.map(f => [f, read(f).replaceAll('\r', '')]));
  function requireText(file, text) {
    if (!contents[file].includes(text)) throw new Error(`Workflow policy missing ${text} in ${file}`);
  }
  const expected = {
    'Taskfile.yml': {
      build: 'task: windows:build', run: 'task: windows:run', package: 'task: windows:package',
      dev: 'wails3 dev -config ./build/config.yml -port {{.VITE_PORT}}',
      'install:msix:tools': 'task: windows:install:msix:tools',
      'setup:docker': 'node scripts/workflow.mjs unsupported',
    },
    'build/windows/Taskfile.yml': {
      build: 'bash scripts/build.sh', run: 'bash scripts/run.sh',
      'dev:build': 'bash scripts/dev.sh build', 'dev:run': 'bash scripts/dev.sh run',
      unsupported: 'node scripts/workflow.mjs unsupported',
      ...Object.fromEntries(['build:native', 'build:docker', 'package', 'create:nsis:installer',
        'create:msix:package', 'install:msix:tools', 'sign', 'sign:installer'].map(k => [k, 'task: unsupported'])),
    },
  };
  for (const [file, mappings] of Object.entries(expected)) {
    const blocks = taskBlocks(contents[file]);
    if (blocks.size !== Object.keys(mappings).length) throw new Error(`Unreviewed task entry in ${file}`);
    for (const [name, command] of Object.entries(mappings)) {
      const block = blocks.get(name) ?? '';
      const commands = [...block.matchAll(/^      - (.+)$/gm)].map(m => m[1]);
      if (JSON.stringify(commands) !== JSON.stringify([command]) || /\b(deps|preconditions|includes|dir|sources|status|generates):/.test(block)) {
        throw new Error(`Unsafe task route: ${file}:${name}`);
      }
    }
  }
  const includes = contents['Taskfile.yml'].split(/^includes:\s*$/m)[1]?.split(/^tasks:/m)[0]?.trim();
  if (includes !== 'common: ./build/Taskfile.yml\n  windows: ./build/windows/Taskfile.yml') throw new Error('Unreviewed platform includes');
  if (/^includes:/m.test(contents['build/windows/Taskfile.yml'])) throw new Error('Nested platform include not permitted');
  for (const file of ['Taskfile.yml', 'build/windows/Taskfile.yml', 'build/Taskfile.yml']) {
    if (/\b(?:go|garble)\s+build\b|\bdocker\s+(?:run|build)\b/.test(contents[file])) throw new Error(`Unmanaged compiler in ${file}`);
  }
  requireText('build/config.yml', 'cmd: wails3 task windows:dev:build');
  requireText('build/config.yml', 'cmd: wails3 task windows:dev:run');
  if (/cmd: wails3 (?:build|task run)/.test(contents['build/config.yml'])) throw new Error('Hot reload routes through delivery');
  for (const text of ['bash scripts/build.sh', 'node scripts/artifacts.mjs audit', '最近已验收', '用户明确验收']) requireText('AGENTS.md', text);
  for (const text of ['node scripts/workflow.mjs environment', 'node scripts/workflow.mjs check',
    'node scripts/artifacts.mjs audit --allow-empty', 'bash scripts/verify.sh',
    'node scripts/artifacts.mjs register', 'node scripts/artifacts.mjs clean --apply',
    'node scripts/artifacts.mjs audit\n']) requireText('scripts/build.sh', text);
  const build = contents['scripts/build.sh'];
  const order = ['audit --allow-empty', 'bash scripts/verify.sh', '\ngo build ', 'artifacts.mjs register', 'artifacts.mjs clean --apply', 'artifacts.mjs audit\n'];
  const indices = order.map(s => build.indexOf(s));
  if (indices.some((n, i) => n < 0 || (i > 0 && n <= indices[i - 1]))) throw new Error('Delivery step order changed');
  requireText('scripts/run.sh', 'node scripts/artifacts.mjs audit');
  requireText('scripts/run.sh', 'node scripts/artifacts.mjs current');
  requireText('scripts/dev.sh', '-o artifacts/hot-reload/leaguepingo-hot-reload.exe');
  if (/scripts\/build\.sh|artifacts\.mjs (?:register|clean|accept)|-tags production/.test(contents['scripts/dev.sh'])) throw new Error('Hot reload must not promote/rotate delivery');
  requireText('scripts/verify.sh', 'node scripts/workflow.mjs check');
  requireText('scripts/verify.sh', 'scripts/workflow.test.mjs');
  requireText('scripts/verify.sh', 'task_scope=${1:-windows}');
  requireText('scripts/verify.sh', 'windows) export LEAGUEPINGO_VERIFY_SCOPE=windows');
  for (const file of ['scripts/build.sh', 'scripts/verify.sh', 'scripts/dev.sh', 'scripts/run.sh']) {
    requireText(file, 'source "$task_root/scripts/env.sh"');
  }
  requireText('docs/ci/verify.yml.example', 'bash scripts/build.sh bin/leaguepingo-ci.exe');
  return { ok: true, checked: policyFiles.length };
}

export function checkEnvironment(env = process.env, platform = process.platform, arch = process.arch) {
  if (platform !== 'win32' || arch !== 'x64') throw new Error('Only native Windows amd64 delivery is supported');
  const constraints = { GOOS: ['windows'], GOARCH: ['amd64'], LEAGUEPINGO_TASK_OS: ['windows'],
    LEAGUEPINGO_TASK_ARCH: ['amd64'], LEAGUEPINGO_TASK_DEV: ['false'], DEV: ['false'],
    LEAGUEPINGO_TASK_OBFUSCATED: ['false'], OBFUSCATED: ['false'],
    LEAGUEPINGO_TASK_TAGS: [], EXTRA_TAGS: [], GOFLAGS: [], GARBLE_ARGS: [] };
  for (const [key, allowed] of Object.entries(constraints)) {
    if (env[key] && !allowed.includes(env[key])) throw new Error(`Unsupported build override: ${key}; use documented native entry points`);
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const command = process.argv[2] ?? 'check';
    if (command === 'check') console.log(JSON.stringify(checkWorkflow(f => fs.readFileSync(path.join(root, f), 'utf8'))));
    else if (command === 'environment') checkEnvironment();
    else if (command === 'unsupported') throw new Error('Packaging/signing/cross-platform template entry disabled; use bash scripts/build.sh for Windows delivery');
    else if (command === 'dev-check') {
      const m = manager(root);
      m.safe('bin/.build-lock');
      const binary = m.safe('artifacts/hot-reload/leaguepingo-hot-reload.exe');
      if (runningExecutables().some(p => p === binary.toLowerCase())) throw new Error('Hot-reload output is still running; no process will be stopped automatically');
    } else throw new Error('Unknown workflow command');
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
