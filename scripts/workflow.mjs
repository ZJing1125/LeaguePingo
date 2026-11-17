import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { manager, runningExecutables } from './artifacts.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const policyFiles = ['AGENTS.md', 'Taskfile.yml', 'build/windows/Taskfile.yml',
  'build/Taskfile.yml', 'build/config.yml', 'scripts/build.sh', 'scripts/run.sh',
  'scripts/dev.sh', 'scripts/verify.sh', 'scripts/audit-player-data.sh', 'docs/ci/verify.yml.example',
  'scripts/package-windows.sh', 'scripts/sign-windows.sh', 'scripts/verify-installer.sh', 'scripts/version.mjs',
  'frontend/package.json', 'frontend/preview/package.json', 'frontend/preview/histoire.config.ts',
  '.github/workflows/ci.yml', '.github/dependabot.yml', 'go.mod'];

// A deliberately restricted policy check, not a general YAML parser. Keep
// entry tasks simple and also test their expansion with the real Wails runner.
function taskBlocks(text, section = 'tasks') {
  const tasks = text.split(new RegExp(`^${section}:\\s*$`, 'm'))[1];
  if (!tasks) throw new Error(`Missing ${section} section`);
  const result = new Map();
  const matches = [...tasks.matchAll(/^  ([\w:]+):\s*$/gm)];
  matches.forEach((m, i) => {
    if (result.has(m[1])) throw new Error(`Duplicate task: ${m[1]}`);
    result.set(m[1], tasks.slice(m.index + m[0].length, matches[i + 1]?.index ?? tasks.length));
  });
  return result;
}

function downloadPolicy(ci) {
  const windows = taskBlocks(ci, 'jobs').get('windows') || '';
  const blocks = windows.split(/^      - /m).slice(1);
  const gate = "        if: ${{ success() && github.ref == 'refs/heads/dev' && (github.event_name == 'push' || (github.event_name == 'workflow_dispatch' && inputs.upload_exe)) }}";
  const steps = ['Prepare development download', 'Upload development download', 'Publish download link'].map(name => {
    const matches = blocks.filter(block => block.startsWith('name: ' + name + '\n'));
    if (matches.length !== 1) throw new Error('Required download step missing or duplicated: ' + name);
    return matches[0];
  });
  for (const block of steps.slice(0, 2)) {
    if (!block.split('\n').includes(gate) || /continue-on-error:/.test(block)) throw new Error('Downloads require successful dev push or explicit manual dev upload');
  }
  const required = [
    ['        id: bundle', '        run: node scripts/ci-download.mjs'],
    ['        id: download', '          name: ${{ steps.bundle.outputs.artifact_name }}', '          path: artifacts/ci-download/', '          if-no-files-found: error', '          retention-days: 14'],
    ["        if: ${{ success() && steps.download.outcome == 'success' }}", '          DOWNLOAD_URL: ${{ steps.download.outputs.artifact-url }}'],
  ];
  steps.forEach((block, i) => {
    if (required[i].some(line => !block.split('\n').includes(line))) throw new Error('Download contents, retention and link must stay explicit');
  });
  const order = ['name: Verify and build (scripts/build.sh)', 'name: Package NSIS installer',
    'name: Verify NSIS install and uninstall', 'name: Check generated bindings are committed', ...steps];
  if (order.some((value, i) => windows.indexOf(value) < 0 || (i && windows.indexOf(value) <= windows.indexOf(order[i - 1])))) {
    throw new Error('Downloads must follow build, installer and binding verification');
  }
}

function qualityPolicy(ci) {
  const jobs = taskBlocks(ci, 'jobs');
  const quality = jobs.get('quality') || '', windows = jobs.get('windows') || '';
  if (jobs.size !== 4 || !jobs.has('portable') || !jobs.has('release') || !quality || !windows ||
      !/^    needs: portable$/m.test(quality) || !/^    needs: \[portable, quality\]$/m.test(windows) ||
      /^    (?:if|continue-on-error):/m.test(quality + windows) || /continue-on-error:/.test(quality)) {
    throw new Error('Windows must depend on mandatory portable and quality jobs');
  }
  if (['portable', 'quality', 'windows'].some(name => (jobs.get(name).match(/^\s*go-version-file: go\.mod$/gm) || []).length !== 1)) {
    throw new Error('Each CI job must read the single Go version source');
  }
  const blocks = quality.split(/^      - /m).slice(1);
  function step(id) {
    const found = blocks.filter(block => block.split('\n').includes('        id: ' + id));
    if (found.length !== 1) throw new Error('Required quality step missing or duplicated: ' + id);
    return found[0];
  }
  const frontend = step('frontend'), analyzers = step('analyzers');
  const prepare = 'name: Prepare embedded frontend for analyzers\n        id: frontend\n        run: |\n          npm --prefix frontend ci\n          npm --prefix frontend run build\n';
  if (!frontend.startsWith(prepare)) throw new Error('Quality analyzers require a locked frontend build in their own job');
  for (const line of ['        run: |', '          tool_bin="$RUNNER_TEMP/quality-bin"',
    '          GOBIN="$tool_bin" go install honnef.co/go/tools/cmd/staticcheck@v0.8.2',
    '          GOBIN="$tool_bin" go install golang.org/x/vuln/cmd/govulncheck@v1.8.0',
    '          echo "$tool_bin" >> "$GITHUB_PATH"']) {
    if (!analyzers.split('\n').includes(line)) throw new Error('Quality analyzers must install the pinned tools');
  }
  const afterFrontend = "${{ !cancelled() && steps.frontend.outcome == 'success' }}";
  const afterTools = "${{ !cancelled() && steps.frontend.outcome == 'success' && steps.analyzers.outcome == 'success' }}";
  for (const [id, command, condition] of [['staticcheck', 'staticcheck ./...', afterTools],
    ['govulncheck', 'govulncheck ./...', afterTools], ['npm_audit', 'npm --prefix frontend audit --audit-level=high', afterFrontend]]) {
    const block = step(id), lines = block.split('\n');
    if (!lines.includes('        run: ' + command) || !lines.includes('        if: ' + condition) ||
        quality.indexOf(frontend) > quality.indexOf(block) || quality.indexOf(analyzers) > quality.indexOf(block)) {
      throw new Error('Required quality checks must run independently after preparation without masking failures');
    }
    if (id !== 'npm_audit' && ['          GOOS: windows', '          GOARCH: amd64', '          CGO_ENABLED: "0"'].some(line => !lines.includes(line))) {
      throw new Error('Quality Go analyzers must cover the Windows target');
    }
  }
  const summary = step('summary');
  if (!summary.split('\n').includes('        if: always()') ||
      ['staticcheck', 'govulncheck', 'npm_audit'].some(id => !summary.includes('${{ steps.' + id + '.outcome }}'))) {
    throw new Error('Quality summary must preserve actual outcomes after failure');
  }
}

function releasePolicy(ci) {
  const jobs = taskBlocks(ci, 'jobs'), release = jobs.get('release') || '';
  for (const line of ['    needs: [portable, quality, windows]',
    "    if: ${{ success() && github.ref == 'refs/heads/dev' && (github.event_name == 'push' || (github.event_name == 'workflow_dispatch' && inputs.upload_exe)) }}",
    '    permissions:', '      actions: read', '      contents: write',
    '          ARTIFACT_NAME: ${{ needs.windows.outputs.download_name }}',
    '          gh run download "$GITHUB_RUN_ID" --repo "$GITHUB_REPOSITORY" --name "$ARTIFACT_NAME" --dir artifacts/release',
    '        run: node scripts/publish-release.mjs']) {
    if (!release.split('\n').includes(line)) throw new Error('Release must publish only the verified dev bundle after all checks');
  }
  if (/continue-on-error:|--clobber|go-version-file:/.test(release) ||
      !/^      download_name: \$\{\{ steps.bundle.outputs.artifact_name \}\}$/m.test(jobs.get('windows')) ||
      (ci.match(/^\s+contents: write$/gm) || []).length !== 1 ||
      [...jobs].some(([name, block]) => name !== 'release' && /contents: write|GH_TOKEN:/.test(block))) {
    throw new Error('Only the release job may write releases; build jobs remain read-only');
  }
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
      ...Object.fromEntries(['build:native', 'build:docker', 'create:msix:package',
        'install:msix:tools'].map(k => [k, 'task: unsupported'])),
      package: 'bash scripts/package-windows.sh',
      'create:nsis:installer': 'bash scripts/package-windows.sh',
      sign: 'bash scripts/sign-windows.sh "{{.FILE}}"',
      'sign:installer': 'bash scripts/sign-windows.sh "{{.FILE}}"',
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
  requireText('scripts/build.sh', 'node scripts/version.mjs check');
  requireText('scripts/build.sh', 'bash scripts/sign-windows.sh "$output"');
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
  requireText('scripts/verify.sh', 'scripts/app-icons.test.mjs');
  requireText('scripts/verify.sh', 'scripts/version.test.mjs');
  requireText('scripts/verify.sh', 'npm --prefix frontend run test:unit');
  requireText('scripts/verify.sh', 'go run ./cmd/functioncheck');
  requireText('scripts/verify.sh', 'node scripts/check-styles.mjs');
  requireText('scripts/verify.sh', 'scripts/check-styles.test.mjs');
  requireText('scripts/verify.sh', 'node scripts/check-copy.mjs');
  requireText('scripts/verify.sh', 'scripts/check-copy.test.mjs');
  requireText('scripts/verify.sh', 'CGO_ENABLED=1 go test -race ./...');
  if (JSON.parse(contents['frontend/package.json']).scripts['test:unit'] !== 'vue-tsc --project tsconfig.unit.json && vitest run') throw new Error('Frontend unit tests must remain a type-checked non-browser gate');
  for (const text of ['source "$task_root/scripts/env.sh"', 'go env GOHOSTOS', 'go env GOHOSTARCH',
    'LEAGUEPINGO_LIVE_DATA_AUDIT=1', "-run '^TestLivePlayerDataAudit$' -count=1 -v"]) requireText('scripts/audit-player-data.sh', text);
  if (/\bgo\s+build\b|scripts\/build\.sh|artifacts\.mjs (register|accept|clean)/.test(contents['scripts/audit-player-data.sh'])) throw new Error('Live audit cannot build/promote delivery artifacts');
  requireText('build/Taskfile.yml', 'node ../scripts/app-icons.mjs generate');
  requireText('scripts/dev.sh', 'node scripts/app-icons.mjs check');
  for (const file of ['scripts/verify.sh', 'scripts/dev.sh']) {
    requireText(file, 'wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/wails.exe.manifest -info build/windows/info.json -out wails_windows_amd64.syso');
  }
  requireText('scripts/verify.sh', 'task_scope=${1:-windows}');
  requireText('scripts/verify.sh', 'windows) export LEAGUEPINGO_VERIFY_SCOPE=windows');
  for (const file of ['scripts/build.sh', 'scripts/verify.sh', 'scripts/dev.sh', 'scripts/run.sh']) {
    requireText(file, 'source "$task_root/scripts/env.sh"');
  }
  for (const file of ['scripts/package-windows.sh', 'scripts/sign-windows.sh', 'scripts/verify-installer.sh']) {
    requireText(file, 'source "$task_root/scripts/env.sh"');
    requireText(file, 'node scripts/workflow.mjs environment');
  }
  requireText('scripts/package-windows.sh', 'bash scripts/build.sh "$task_binary"');
  requireText('scripts/package-windows.sh', 'node scripts/artifacts.mjs current');
  requireText('scripts/package-windows.sh', 'node scripts/environment.mjs nsis');
  requireText('scripts/package-windows.sh', 'bash scripts/sign-windows.sh "$task_output"');
  requireText('scripts/sign-windows.sh', '"$task_signtool" verify');
  requireText('scripts/verify-installer.sh', 'NOT RUN: clean Windows 10');
  requireText('scripts/verify-installer.sh', 'Copy-Item -LiteralPath $env:LEAGUEPINGO_VERIFY_EXECUTABLE -Destination $copy');
  requireText('scripts/verify-installer.sh', '-FilePath $copy -ArgumentList "/S _?=$env:LEAGUEPINGO_VERIFY_INSTALL_ROOT"');
  requireText('scripts/verify-installer.sh', '$process.WaitForExit(60000)');
  requireText('docs/ci/verify.yml.example', 'bash scripts/build.sh bin/leaguepingo-ci.exe');
  // Fixed CI entry points. The Windows job delegates verification to build.sh
  // exactly once; a green portable job is never desktop verification.
  const ci = contents['.github/workflows/ci.yml'];
  const actions = [...ci.matchAll(/^\s*- uses:\s*(\S+)/gm)].map(m => m[1]);
  if (!actions.length || actions.some(action => !/^actions\/(checkout|setup-go|setup-node|cache|upload-artifact)@[a-f0-9]{40}$/.test(action))) {
    throw new Error('CI actions must use reviewed official actions pinned to a full commit SHA');
  }
  const dependencies = contents['.github/dependabot.yml'];
  for (const text of ['version: 2', 'multi-ecosystem-groups:', 'package-ecosystem: gomod', 'package-ecosystem: npm', 'package-ecosystem: github-actions', 'directory: "/frontend"', 'directory: "/frontend/preview"', 'timezone: Asia/Shanghai']) {
    requireText('.github/dependabot.yml', text);
  }
  const updates = dependencies.split(/^  - package-ecosystem:/m).slice(1);
  if (updates.length !== 4 || updates.some(block => !/^    target-branch: dev$/m.test(block)) ||
      (dependencies.match(/^    multi-ecosystem-group: desktop$/gm) || []).length !== 2) {
    throw new Error('Dependency updates must target dev and review product Go/npm dependencies together');
  }
  for (const text of ['branches: [dev]', 'branches: [dev, main]', 'contents: read',
    'needs: portable', 'runs-on: ubuntu-latest', 'runs-on: windows-latest',
    'run: bash scripts/verify.sh portable', 'git status --porcelain -- frontend/bindings',
    'go install honnef.co/go/tools/cmd/staticcheck@v0.8.2',
    'go install golang.org/x/vuln/cmd/govulncheck@v1.8.0',
    'run: staticcheck ./...', 'run: govulncheck ./...',
    'run: npm --prefix frontend audit --audit-level=high', 'GOTOOLCHAIN: local',
    'choco install nsis --version=3.13.0', 'bash scripts/package-windows.sh --binary bin/leaguepingo-ci.exe',
    'bash scripts/verify-installer.sh artifacts/installer-check/leaguepingo-ci-installer.exe',
    "github.event_name == 'workflow_dispatch' && inputs.upload_exe"])
    requireText('.github/workflows/ci.yml', text);
  const declaredGo = contents['go.mod'].match(/^go (\d+)\.(\d+)\.(\d+)$/m);
  const goParts = declaredGo?.slice(1).map(Number) || [];
  const goFloor = [1, 26, 11];
  const firstDifference = goParts.findIndex((value, index) => value !== goFloor[index]);
  if (!declaredGo || (firstDifference >= 0 && goParts[firstDifference] < goFloor[firstDifference]) ||
      /^toolchain /m.test(contents['go.mod']) || /^\s*go-version:/m.test(ci) ||
      [...ci.matchAll(/^\s*GOTOOLCHAIN:\s*(\S+)/gm)].some(match => match[1] !== 'local') ||
      (ci.match(/^\s*go-version-file: go\.mod$/gm) || []).length !== 3) {
    throw new Error('CI Go versions must use go.mod with the patched Go 1.26.11 minimum');
  }
  qualityPolicy(ci);
  downloadPolicy(ci);
  releasePolicy(ci);
  requireText('scripts/verify.sh', 'scripts/ci-download.test.mjs');
  requireText('scripts/verify.sh', 'scripts/publish-release.test.mjs');
  if ((ci.match(/run: bash scripts\/build\.sh bin\/leaguepingo-ci\.exe/g) || []).length !== 1 ||
      /^\s*(?:run: |.*&&\s*)(?:go|garble)\s+build\b/m.test(ci) ||
      /run: bash scripts\/verify\.sh(?:\s*\n|\s+windows)/.test(ci) ||
      /^\s*pull_request_target:/m.test(ci)) {
    throw new Error('CI must use portable checks followed by one managed Windows build');
  }
  const frontendScripts = JSON.parse(contents['frontend/package.json']).scripts;
  const previewScripts = JSON.parse(contents['frontend/preview/package.json']).scripts;
  const previewRoutes = {
    'preview:components': 'npm --prefix preview run dev',
    'build:components': 'npm --prefix preview run build',
    'test:components': 'node tests/component-preview.cjs',
  };
  for (const [name, command] of Object.entries(previewRoutes)) {
    if (frontendScripts[name] !== command) throw new Error(`Unsafe component preview route: ${name}`);
  }
  if (JSON.stringify(Object.keys(previewScripts).sort()) !== JSON.stringify(['build', 'dev']) ||
      previewScripts.build !== 'histoire build' || previewScripts.dev !== 'histoire dev --host 127.0.0.1') {
    throw new Error('Component preview must remain isolated from desktop delivery');
  }
  requireText('frontend/preview/histoire.config.ts', 'outDir: "../../artifacts/component-preview"');
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
    else if (command === 'unsupported') throw new Error('This cross-platform or MSIX entry is disabled; use documented native Windows entry points');
    else if (command === 'dev-check') {
      const m = manager(root);
      m.safe('bin/.build-lock');
      const binary = m.safe('artifacts/hot-reload/leaguepingo-hot-reload.exe');
      if (runningExecutables().some(p => p === binary.toLowerCase())) throw new Error('Hot-reload output is still running; no process will be stopped automatically');
    } else throw new Error('Unknown workflow command');
  } catch (error) { console.error(error.message); process.exitCode = 1; }
}
