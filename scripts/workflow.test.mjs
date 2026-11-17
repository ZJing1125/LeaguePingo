import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { checkWorkflow, checkEnvironment, policyFiles } from './workflow.mjs';
import { manager } from './artifacts.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const files = Object.fromEntries(policyFiles.map(f => [f, fs.readFileSync(path.join(root, f), 'utf8')]));
const nativeOnly = { skip: process.platform !== 'win32' || process.env.LEAGUEPINGO_VERIFY_SCOPE === 'portable' };
test('repository workflow policy is consistent', () => {
  assert.equal(checkWorkflow(f => files[f]).ok, true);
});
test('policy regression mutations are rejected', () => {
  for (const [file, before, after] of [
    ['Taskfile.yml', 'task: windows:build', 'go build -o bin/leaguepingo.exe .'],
    ['Taskfile.yml', '  windows: ./build/windows/Taskfile.yml', '  windows: ./build/windows/Taskfile.yml\n  darwin: ./build/darwin/Taskfile.yml'],
    ['build/windows/Taskfile.yml', 'bash scripts/build.sh', 'go build -o bin/leaguepingo.exe .'],
    ['build/windows/Taskfile.yml', '  run:', '  surprise:\n    cmds:\n      - go build .\n  run:'],
    ['build/windows/Taskfile.yml', '      - bash scripts/build.sh', '      - bash scripts/build.sh\n      - go build .'],
    ['build/config.yml', 'cmd: wails3 task windows:dev:build', 'cmd: wails3 build DEV=true'],
    ['scripts/build.sh', 'bash scripts/verify.sh', 'echo skipped verification'],
    ['scripts/build.sh', 'node scripts/artifacts.mjs audit\n', 'echo no audit\n'],
    ['scripts/dev.sh', '-o artifacts/hot-reload/leaguepingo-hot-reload.exe', '-o bin/leaguepingo.exe'],
    ['scripts/verify.sh', 'scripts/workflow.test.mjs', ''],
    ['scripts/verify.sh', 'scripts/version.test.mjs', ''],
    ['scripts/verify.sh', 'scripts/ci-download.test.mjs', ''],
    ['scripts/verify.sh', 'scripts/publish-release.test.mjs', ''],
    ['scripts/verify.sh', 'npm --prefix frontend run test:unit', 'echo skipped unit tests'],
    ['scripts/verify.sh', 'go run ./cmd/functioncheck', 'echo skipped function limit'],
    ['scripts/verify.sh', 'node scripts/check-styles.mjs', 'echo skipped style ownership'],
    ['scripts/verify.sh', 'scripts/check-styles.test.mjs', ''],
    ['scripts/verify.sh', 'node scripts/check-copy.mjs', 'echo skipped copy catalog'],
    ['scripts/verify.sh', 'scripts/check-copy.test.mjs', ''],
    ['scripts/verify.sh', 'CGO_ENABLED=1 go test -race ./...', 'echo skipped race checks'],
    ['scripts/package-windows.sh', 'node scripts/artifacts.mjs current', 'echo unverified'],
    ['scripts/sign-windows.sh', '"$task_signtool" verify', 'echo trusted'],
    ['scripts/verify-installer.sh', '/S _?=$env:LEAGUEPINGO_VERIFY_INSTALL_ROOT', '/S'],
    ['scripts/verify-installer.sh', '$process.WaitForExit(60000)', '$true'],
    ['scripts/audit-player-data.sh', 'LEAGUEPINGO_LIVE_DATA_AUDIT=1', ''],
    ['scripts/audit-player-data.sh', 'go env GOHOSTOS', 'go env GOOS'],
    ['scripts/verify.sh', 'wails3 generate syso -arch amd64', 'echo skipped Windows resources'],
    ['scripts/dev.sh', 'node scripts/app-icons.mjs check', 'echo skipped brand validation'],
    ['build/Taskfile.yml', 'node ../scripts/app-icons.mjs generate', 'echo skipped icon generation'],
    ['AGENTS.md', 'node scripts/artifacts.mjs audit', 'skip audit'],
    ['docs/ci/verify.yml.example', 'bash scripts/build.sh bin/leaguepingo-ci.exe', 'go build .'],
  ]) {
    assert.ok(files[file].includes(before), file);
    const changed = files[file].replace(before, after);
    assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]), undefined, file);
  }
});
test('unsupported environment flags fail instead of silently changing the target', () => {
  assert.doesNotThrow(() => checkEnvironment({}, 'win32', 'x64'));
  for (const env of [{ GOOS: 'darwin' }, { GOARCH: 'arm64' }, { LEAGUEPINGO_TASK_DEV: 'true' },
    { LEAGUEPINGO_TASK_OS: 'linux' }, { EXTRA_TAGS: 'foo' }, { GOFLAGS: '-tags=test' }, { OBFUSCATED: 'true' }]) {
    assert.throws(() => checkEnvironment(env, 'win32', 'x64'));
  }
  assert.throws(() => checkEnvironment({}, 'darwin', 'arm64'));
});
test('CI preserves managed build, binding verification and successful dev download upload', () => {
  const file = '.github/workflows/ci.yml';
  for (const [before, after] of [
    ['run: bash scripts/build.sh bin/leaguepingo-ci.exe', 'run: go build -o bin/leaguepingo-ci.exe .'],
    ['run: bash scripts/build.sh bin/leaguepingo-ci.exe', 'run: bash scripts/verify.sh windows\n      - run: bash scripts/build.sh bin/leaguepingo-ci.exe'],
    ['needs: portable', 'needs: []'],
    ['git status --porcelain -- frontend/bindings', 'git diff -- frontend/bindings'],
    ['go install honnef.co/go/tools/cmd/staticcheck@v0.8.2', 'go install honnef.co/go/tools/cmd/staticcheck@latest'],
    ['go install golang.org/x/vuln/cmd/govulncheck@v1.8.0', 'go install golang.org/x/vuln/cmd/govulncheck@latest'],
    ['run: govulncheck ./...', 'run: true'],
    ['GOTOOLCHAIN: local', 'GOTOOLCHAIN: auto'],
    ["github.event_name == 'workflow_dispatch' && inputs.upload_exe", 'success()'],
    ["success() && github.ref == 'refs/heads/dev'", "github.ref == 'refs/heads/dev'"],
    ["github.ref == 'refs/heads/dev'", "github.ref == 'refs/heads/main'"],
    ["github.event_name == 'push'", "github.event_name == 'pull_request'"],
    ['path: artifacts/ci-download/', 'path: bin/'],
    ['retention-days: 14', 'retention-days: 90'],
    ['run: node scripts/ci-download.mjs', 'run: echo skipped packaging'],
    ['  pull_request:', '  pull_request_target:'],
  ]) {
    assert.ok(files[file].includes(before), before);
    const changed = files[file].replace(before, after);
    assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]), undefined, before);
  }
});
test('dependency updates stay coupled on dev and CI actions stay pinned', () => {
  const ci = '.github/workflows/ci.yml';
  const unpinned = files[ci].replace(/actions\/checkout@[a-f0-9]{40}/, 'actions/checkout@v4');
  assert.notEqual(unpinned, files[ci]);
  assert.throws(() => checkWorkflow(f => f === ci ? unpinned : files[f]));
  const config = '.github/dependabot.yml';
  for (const [before, after] of [
    ['target-branch: dev', 'target-branch: main'],
    ['multi-ecosystem-group: desktop', 'multi-ecosystem-group: separated'],
    ['directory: "/frontend/preview"', 'directory: "/frontend"'],
    ['multi-ecosystem-groups:', 'disabled-groups:'],
  ]) {
    assert.ok(files[config].includes(before), before);
    const changed = files[config].replace(before, after);
    assert.throws(() => checkWorkflow(f => f === config ? changed : files[f]), undefined, before);
  }
});
test('download cannot run before installer and binding verification', () => {
  const file = '.github/workflows/ci.yml';
  const start = files[file].indexOf('      - name: Prepare development download');
  const end = files[file].indexOf('      - name: Upload development download');
  const prepare = files[file].slice(start, end);
  const changed = files[file].replace(prepare, '').replace('      - name: Verify NSIS install and uninstall',
    prepare + '      - name: Verify NSIS install and uninstall');
  assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]), /Downloads must follow/);
});
test('releases cannot publish PRs, skip dependencies or choose an unrelated artifact', () => {
  const file = '.github/workflows/ci.yml';
  for (const [before, after] of [
    ['needs: [portable, quality, windows]', 'needs: [portable]'],
    ['name: Publish development release', 'name: Publish development release\n    continue-on-error: true'],
    ['ARTIFACT_NAME: ${{ needs.windows.outputs.download_name }}', 'ARTIFACT_NAME: arbitrary'],
    ['run: node scripts/publish-release.mjs', 'run: gh release create unsafe'],
    ['      contents: write', '      contents: read'],
    ['      download_name: ${{ steps.bundle.outputs.artifact_name }}', '      download_name: wrong'],
  ]) {
    assert.ok(files[file].includes(before));
    const changed = files[file].replace(before, after);
    assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]));
  }
  const changed = files[file].replace(/(  release:[\s\S]+)$/, block => block.replace("github.ref == 'refs/heads/dev'", 'true'));
  assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]));
});
test('all CI jobs follow one patched Go version source', () => {
  for (const [file, before, after] of [
    ['go.mod', 'go 1.26.11', 'go 1.26.10'],
    ['go.mod', 'go 1.26.11', 'go 1.26.9'],
    ['go.mod', 'go 1.26.11', 'go 1.25.14'],
    ['go.mod', 'go 1.26.11', 'go 1.26.11\ntoolchain go1.27.0'],
    ['.github/workflows/ci.yml', 'go-version-file: go.mod', 'go-version: "1.25.x"'],
    ['.github/workflows/ci.yml', 'go-version-file: go.mod', 'go-version-file: other.mod'],
    ['.github/workflows/ci.yml', 'name: Quality checks', 'name: Quality checks\n    env:\n      GOTOOLCHAIN: auto'],
  ]) {
    assert.ok(files[file].includes(before));
    const changed = files[file].replace(before, after);
    assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]), /patched Go 1.26.11 minimum/);
  }
});
test('quality analyzers prepare real embedded assets in their own job', () => {
  const file = '.github/workflows/ci.yml';
  const prepare = '      - name: Prepare embedded frontend for analyzers\n        id: frontend\n        run: |\n          npm --prefix frontend ci\n          npm --prefix frontend run build\n';
  assert.ok(files[file].includes(prepare));
  for (const changed of [
    files[file].replace(prepare, ''),
    files[file].replace(prepare, prepare.replace('npm --prefix frontend ci', 'npm --prefix frontend install')),
    files[file].replace(prepare, prepare.replace('npm --prefix frontend run build', 'mkdir -p frontend/dist')),
    // A build in the portable job cannot supply another checkout's embed inputs.
    files[file].replace(prepare, '').replace('      - name: Portable verification', prepare + '\n      - name: Portable verification'),
    files[file].replace(prepare, '').replace('      - name: npm audit', prepare + '\n      - name: npm audit'),
  ]) {
    assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]));
  }
});
test('quality failures block Windows and do not suppress independent check outcomes', () => {
  const file = '.github/workflows/ci.yml';
  const original = files[file];
  const afterTools = "${{ !cancelled() && steps.frontend.outcome == 'success' && steps.analyzers.outcome == 'success' }}";
  for (const [before, after] of [
    ['needs: [portable, quality]', 'needs: portable'],
    ['    name: Quality checks', '    name: Quality checks\n    if: false'],
    ['    name: Windows verify and build', '    name: Windows verify and build\n    if: always()'],
    ['        id: staticcheck', '        id: staticcheck\n        continue-on-error: true'],
    ['        id: govulncheck', '        id: govulncheck\n        continue-on-error: true'],
    ['        id: npm_audit', '        id: npm_audit\n        continue-on-error: true'],
    ['run: staticcheck ./...', 'run: staticcheck ./... || true'],
    ['run: govulncheck ./...', 'run: govulncheck ./... || true'],
    ['run: npm --prefix frontend audit --audit-level=high', 'run: npm --prefix frontend audit --audit-level=high || true'],
    ['        if: ' + afterTools, '        if: success()'],
    ["        if: ${{ !cancelled() && steps.frontend.outcome == 'success' }}", '        if: success()'],
    ['        id: frontend', '        id: missing_frontend'],
    ['        id: analyzers', '        id: missing_analyzers'],
    ['          GOOS: windows', '          GOOS: linux'],
    ['        if: always()', '        if: success()'],
    ['${{ steps.govulncheck.outcome }}', 'success'],
  ]) {
    assert.ok(original.includes(before), before);
    const changed = original.replace(before, after);
    assert.throws(() => checkWorkflow(f => f === file ? changed : files[f]), undefined, before);
  }
  const tools = original.match(/      - name: Install pinned analyzers\n[\s\S]*?(?=      - name: Staticcheck)/)?.[0];
  assert.ok(tools);
  const delayedTools = original.replace(tools, '').replace('      - name: npm audit', tools + '      - name: npm audit');
  assert.throws(() => checkWorkflow(f => f === file ? delayedTools : files[f]), /independently after preparation/);
});

test('component preview routes cannot become product or desktop delivery entry points', () => {
  for (const [file, change] of [
    ['frontend/package.json', json => { json.scripts['build:components'] = 'wails3 build'; }],
    ['frontend/preview/package.json', json => { json.scripts.build = 'vite build --outDir ../dist'; }],
    ['frontend/preview/package.json', json => { json.scripts.package = 'go build -o bin/preview.exe'; }],
  ]) {
    const json = JSON.parse(files[file]);
    change(json);
    assert.throws(() => checkWorkflow(f => f === file ? JSON.stringify(json) : files[f]));
  }
  assert.throws(() => checkWorkflow(f => f === 'frontend/preview/histoire.config.ts'
    ? files[f].replace('../../artifacts/component-preview', '../dist') : files[f]));
});

test('real Wails dry task expansion uses reviewed scripts; disabled tasks have no compiler deps', nativeOnly, () => {
  for (const [name, expected] of [
    ['build', 'bash scripts/build.sh'], ['windows:build', 'bash scripts/build.sh'],
    ['run', 'bash scripts/run.sh'], ['windows:run', 'bash scripts/run.sh'],
    ['windows:dev:build', 'bash scripts/dev.sh build'], ['windows:dev:run', 'bash scripts/dev.sh run'],
    ['dev', 'wails3 dev -config ./build/config.yml'],
    ['package', 'bash scripts/package-windows.sh'], ['windows:package', 'bash scripts/package-windows.sh'],
    ['windows:create:nsis:installer', 'bash scripts/package-windows.sh'],
    ['windows:sign', 'bash scripts/sign-windows.sh'], ['windows:sign:installer', 'bash scripts/sign-windows.sh'],
    ...['windows:build:native', 'windows:build:docker', 'windows:create:msix:package',
      'install:msix:tools', 'setup:docker', 'common:setup:docker']
      .map(n => [n, 'node scripts/workflow.mjs unsupported']),
  ]) {
    const result = spawnSync('wails3', ['task', '-dry', name], { cwd: root, encoding: 'utf8', timeout: 15000 });
    assert.equal(result.status, 0, `${name}: ${result.error ?? ''} ${result.stderr}`);
    const output = result.stdout + result.stderr;
    assert.ok(output.includes(expected), output);
    assert.doesNotMatch(output, /\bgo build\b|\bgarble\b|\bdocker (?:run|build)\b|\bmakensis\b|go mod tidy|npm install/);
  }
});
test('package and sign commands fail closed when explicit inputs are missing', nativeOnly, () => {
  const packageResult = spawnSync('wails3', ['task', 'package'], { cwd: root, encoding: 'utf8', timeout: 15000,
    env: { ...process.env, LEAGUEPINGO_MAKENSIS: 'Z:/definitely-missing/makensis.exe' } });
  assert.notEqual(packageResult.status, 0);
  const signResult = spawnSync('wails3', ['task', 'windows:sign', 'FILE=missing.exe'], { cwd: root, encoding: 'utf8', timeout: 15000 });
  assert.notEqual(signResult.status, 0);
});
test('Wails alias and flags route correctly in an isolated mock workspace', nativeOnly, t => {
  const { folder } = fixture(t);
  for (const file of ['Taskfile.yml', 'build/Taskfile.yml', 'build/windows/Taskfile.yml', 'build/config.yml', 'scripts/workflow.mjs', 'scripts/artifacts.mjs']) {
    fs.mkdirSync(path.dirname(path.join(folder, file)), { recursive: true });
    fs.copyFileSync(path.join(root, file), path.join(folder, file));
  }
  // Never execute the real build script from a task-routing test.
  fs.writeFileSync(path.join(folder, 'scripts/build.sh'), 'set -e\nnode scripts/workflow.mjs environment\necho MOCK_DELIVERY_REACHED\n');
  for (const args of [['build'], ['task', 'build'], ['task', 'windows:build']]) {
    const result = spawnSync('wails3', args, { cwd: folder, encoding: 'utf8', timeout: 15000 });
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.match(result.stdout, /MOCK_DELIVERY_REACHED/);
  }
  for (const flag of ['DEV=true', 'GOOS=darwin', 'GOARCH=arm64']) {
    const result = spawnSync('wails3', ['build', flag], { cwd: folder, encoding: 'utf8', timeout: 15000 });
    assert.notEqual(result.status, 0, `${flag}: ${result.stdout} ${result.stderr}`);
    assert.match(result.stdout + result.stderr, /Unsupported build override/);
  }
});

function fixture(t, running = () => []) {
  const folder = fs.mkdtempSync(path.join(os.tmpdir(), 'leaguepingo-workflow-test-'));
  t.after(() => {
    assert.ok(path.basename(folder).startsWith('leaguepingo-workflow-test-'));
    fs.rmSync(folder, { recursive: true, force: true });
  });
  fs.mkdirSync(path.join(folder, 'bin'));
  const m = manager(folder, () => running(folder));
  const builds = [1, 2].map(n => {
    const file = `bin/leaguepingo-v${n}.exe`;
    fs.writeFileSync(path.join(folder, file), `synthetic-${n}`);
    return { file, accepted: true };
  });
  m.bootstrap({ builds, retired: [] });
  return { folder, m };
}
test('delivery audit verifies hashes and detects unregistered nested binaries without deleting them', t => {
  const { folder, m } = fixture(t);
  assert.equal(m.audit().current.file, 'bin/leaguepingo-v2.exe');
  fs.mkdirSync(path.join(folder, 'bin/rogue'));
  const rogue = path.join(folder, 'bin/rogue/custom.EXE');
  fs.writeFileSync(rogue, 'unknown');
  assert.throws(() => m.audit(), /Unmanaged binaries/);
  assert.ok(fs.existsSync(rogue));
  fs.unlinkSync(rogue);
  fs.appendFileSync(path.join(folder, 'bin/leaguepingo-v2.exe'), 'changed');
  assert.throws(() => m.audit(), /Changed file/);
});
test('audit allows excess only while the registered old program is running', t => {
  let busy = true;
  const { folder, m } = fixture(t, p => busy ? [path.join(p, 'bin/leaguepingo-v1.exe')] : []);
  const file = 'bin/leaguepingo-v3.exe';
  m.prepare(file);
  const pe = Buffer.alloc(256); pe.write('MZ'); pe.writeUInt32LE(128, 60); pe.set([80,69,0,0], 128);
  fs.writeFileSync(path.join(folder, file), pe);
  m.register(file, 'abcdef1');
  assert.deepEqual(m.audit().skippedRunning, ['bin/leaguepingo-v1.exe']);
  busy = false;
  assert.throws(() => m.audit(), /pending cleanup/);
  m.clean(true);
  assert.equal(m.audit().retained.length, 2);
});
test('audit supports an empty clone only explicitly, without creating a manifest', t => {
  const { folder, m } = fixture(t);
  for (const file of fs.readdirSync(path.join(folder, 'bin'))) fs.unlinkSync(path.join(folder, 'bin', file));
  assert.equal(m.audit(true).current, null);
  assert.throws(() => m.audit(), /Missing manifest/);
  assert.ok(!fs.existsSync(path.join(folder, 'bin/artifacts.json')));
});
