import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const project = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const digest = file => createHash('sha256').update(fs.readFileSync(file)).digest('hex');

// Windows-specific, read-only process discovery. Do not read command lines or
// change execution policy. Failure means cleanup must not run.
export function runningExecutables() {
  if (process.platform !== 'win32') throw new Error('Cleanup requires Windows process discovery');
  const output = execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command',
    "$ErrorActionPreference='Stop'; $items=@(Get-CimInstance Win32_Process -Filter \"Name LIKE 'leaguepingo%.exe'\" | Select-Object Name,ExecutablePath); ConvertTo-Json -InputObject $items -Compress"
  ], { encoding: 'utf8', timeout: 15000 }).trim();
  const items = JSON.parse(output);
  if (!Array.isArray(items) || items.some(p => !p.ExecutablePath)) {
    throw new Error('Cannot resolve all running LeaguePingo paths; cleanup refused');
  }
  return items.map(p => path.resolve(p.ExecutablePath).toLowerCase());
}

export function manager(root, processes = runningExecutables) {
  root = fs.realpathSync(root);
  function safe(relative) {
    if (typeof relative !== 'string' || !/^(bin|artifacts)\/[A-Za-z0-9_./-]+$/.test(relative) ||
        relative.split('/').some(p => !p || p === '.' || p === '..')) throw new Error(`Unsafe path: ${relative}`);
    let current = root;
    const parts = relative.split('/');
    for (let i = 0; i < parts.length; i++) {
      current = path.join(current, parts[i]);
      if (!fs.existsSync(current)) {
        // lstat detects dangling links that existsSync would otherwise hide.
        try { if (fs.lstatSync(current).isSymbolicLink()) throw new Error(`Link refused: ${relative}`); }
        catch (e) { if (e.code !== 'ENOENT') throw e; }
        continue;
      }
      const stat = fs.lstatSync(current);
      if (stat.isSymbolicLink() || (i < parts.length - 1 && !stat.isDirectory())) throw new Error(`Link/non-directory refused: ${relative}`);
      const resolved = fs.realpathSync(current);
      if (!resolved.toLowerCase().startsWith((root + path.sep).toLowerCase())) throw new Error(`Outside project: ${relative}`);
    }
    return current;
  }
  function executable(file) {
    if (!/^bin\/leaguepingo[A-Za-z0-9_-]*\.exe$/.test(file)) throw new Error('Expected bin/leaguepingo*.exe');
    return safe(file);
  }
  function evidence(file) {
    if (!/^(?:bin\/[A-Za-z0-9_-]+\.png|bin\/performance-[A-Za-z0-9_-]+\/process-tree-[0-9-]+\.csv|artifacts\/[A-Za-z0-9_-]+\/[A-Za-z0-9_-]+\.(?:png|csv|html))$/.test(file)) throw new Error(`Not managed evidence: ${file}`);
    return safe(file);
  }
  function fingerprint(file) {
    const target = safe(file);
    if (!fs.statSync(target).isFile()) throw new Error(`Not a file: ${file}`);
    return { file, sha256: digest(target) };
  }
  function validate(state) {
    if (state.version !== 1 || !Array.isArray(state.builds) || !Array.isArray(state.retired)) throw new Error('Invalid manifest');
    const seen = new Set();
    function check(item, isExe) {
      if (!item || !/^[a-f0-9]{64}$/.test(item.sha256)) throw new Error('Invalid file hash');
      (isExe ? executable : evidence)(item.file);
      const key = item.file.toLowerCase();
      if (seen.has(key)) throw new Error(`Duplicate managed path: ${item.file}`);
      seen.add(key);
    }
    for (const b of state.builds) {
      check(b, true);
      if (typeof b.accepted !== 'boolean' || !Array.isArray(b.evidence)) throw new Error('Invalid build');
      b.evidence.forEach(e => check(e, false));
    }
    state.retired.forEach(e => check(e, false));
    return state;
  }
  const manifest = 'bin/artifacts.json';
  function read() { return validate(JSON.parse(fs.readFileSync(safe(manifest), 'utf8'))); }
  function save(state) {
    validate(state);
    const temp = safe('bin/artifacts-next.json');
    fs.writeFileSync(temp, JSON.stringify(state, null, 2) + '\n', { flag: 'wx' });
    fs.renameSync(temp, safe(manifest));
  }
  function verified(item, missingOK = false) {
    const target = safe(item.file);
    if (!fs.existsSync(target) && missingOK) return false;
    if (digest(target) !== item.sha256) throw new Error(`Changed file; refused: ${item.file}`);
    return true;
  }
  function keep(state) {
    const latest = state.builds.at(-1);
    if (!latest) throw new Error('No current build');
    const previous = state.builds.slice(0, -1);
    const rollback = previous.findLast(b => b.accepted) ?? previous.at(-1);
    return new Set([latest.file, rollback?.file].filter(Boolean));
  }
  function clean(apply = false) {
    const state = read();
    const retained = keep(state);
    // Verify both survivors before any deletion. Missing/changed survivors abort.
    for (const b of state.builds.filter(b => retained.has(b.file))) verified(b);
    const running = new Set(processes().map(p => path.resolve(p).toLowerCase()));
    const obsolete = state.builds.filter(b => !retained.has(b.file));
    const busy = obsolete.filter(b => running.has(executable(b.file).toLowerCase()));
    const deletable = obsolete.filter(b => !busy.includes(b));
    const targets = deletable.flatMap(b => [b, ...b.evidence]).concat(busy.length ? [] : state.retired);
    // Preflight the entire batch; do not partially delete on a known hash mismatch.
    for (const item of targets) verified(item, true);
    const report = { retained: [...retained], skippedRunning: busy.map(b => b.file),
      files: targets.filter(i => fs.existsSync(safe(i.file))).map(i => i.file), bytes: 0, applied: apply };
    for (const item of targets) {
      if (!verified(item, true)) continue;
      report.bytes += fs.statSync(safe(item.file)).size;
      if (apply) fs.unlinkSync(safe(item.file)); // Individual files only; never recursive deletion.
    }
    if (apply) {
      state.builds = state.builds.filter(b => retained.has(b.file) || busy.includes(b));
      if (!busy.length) state.retired = [];
      save(state);
      // Remove only empty directories belonging to deleted files. Unknown files,
      // including isolated settings, keep their directories intact.
      for (const item of targets) {
        let dir = path.posix.dirname(item.file);
        while (dir !== 'bin' && dir !== 'artifacts') {
          const target = safe(dir);
          if (!fs.existsSync(target) || fs.readdirSync(target).length) break;
          fs.rmdirSync(target);
          dir = path.posix.dirname(dir);
        }
      }
    }
    return report;
  }
  // Delivery gate is read-only: unknown binaries are reported, never swept.
  function audit(allowEmpty = false) {
    const binaries = [];
    function scan(relative) {
      const target = safe(relative);
      if (!fs.existsSync(target)) return;
      const stat = fs.lstatSync(target);
      if (stat.isDirectory()) {
        for (const name of fs.readdirSync(target)) scan(`${relative}/${name}`);
      } else if (/\.exe$/i.test(relative)) binaries.push(relative);
    }
    // safe() intentionally rejects broad root paths; validate child paths while
    // also refusing a replaced bin directory before walking it.
    safe('bin/artifacts.json');
    if (fs.existsSync(path.join(root, 'bin'))) {
      for (const name of fs.readdirSync(path.join(root, 'bin'))) scan(`bin/${name}`);
    }
    if (!fs.existsSync(safe(manifest))) {
      if (allowEmpty && !binaries.length) return { current: null, retained: [], skippedRunning: [] };
      throw new Error('Missing manifest or unmanaged binaries; delivery refused');
    }
    const state = read();
    const known = new Set(state.builds.map(b => b.file.toLowerCase()));
    const unknown = binaries.filter(f => !known.has(f.toLowerCase()));
    if (unknown.length) throw new Error(`Unmanaged binaries; inspect, do not delete automatically: ${unknown.join(', ')}`);
    for (const b of state.builds) {
      verified(b);
      b.evidence.forEach(e => verified(e));
    }
    if (!state.builds.length) {
      if (allowEmpty && !state.retired.length) return { current: null, retained: [], skippedRunning: [] };
      throw new Error('No delivery build');
    }
    const retained = keep(state);
    const excess = state.builds.filter(b => !retained.has(b.file));
    const running = excess.length ? new Set(processes().map(p => path.resolve(p).toLowerCase())) : new Set();
    if (excess.some(b => !running.has(executable(b.file).toLowerCase())) || state.retired.length) {
      throw new Error('Obsolete artifacts pending cleanup; delivery refused');
    }
    const { file, commit, accepted, sha256 } = state.builds.at(-1);
    return { current: { file, commit, accepted, sha256 }, retained: [...retained], skippedRunning: excess.map(b => b.file) };
  }
  function checkOutput(file) {
    const target = executable(file);
    if (fs.existsSync(target)) throw new Error('Output already exists; never overwrite a build');
    const id = path.basename(file, '.exe');
    const folder = safe(`artifacts/${id}`);
    if (fs.existsSync(folder)) throw new Error('Evidence directory already exists');
    return folder;
  }
  function prepare(file) {
    const folder = checkOutput(file);
    if (!fs.existsSync(safe(manifest))) {
      if (fs.readdirSync(path.join(root, 'bin')).some(f => /\.exe$/i.test(f))) {
        throw new Error('Existing binaries require explicit manifest initialization');
      }
      save({ version: 1, builds: [], retired: [] });
    }
    read();
    fs.mkdirSync(folder, { recursive: true });
    return folder;
  }
  function discard(file) {
    const state = read();
    if (state.builds.some(b => b.file.toLowerCase() === file.toLowerCase())) throw new Error('Cannot discard a registered build');
    const target = executable(file);
    if (processes().some(p => path.resolve(p).toLowerCase() === target.toLowerCase())) throw new Error('Failed output is running; preserved');
    const folder = `artifacts/${path.basename(file, '.exe')}`;
    const files = fs.existsSync(safe(folder)) ? fs.readdirSync(safe(folder)).map(f => `${folder}/${f}`) : [];
    files.forEach(evidence);
    for (const f of files) fs.unlinkSync(safe(f));
    if (fs.existsSync(target)) fs.unlinkSync(target);
    if (fs.existsSync(safe(folder))) fs.rmdirSync(safe(folder));
  }
  function register(file, commit) {
    const state = read();
    if (state.builds.some(b => b.file.toLowerCase() === file.toLowerCase())) throw new Error('Already registered');
    const target = executable(file);
    const buffer = fs.readFileSync(target);
    const offset = buffer.length >= 64 ? buffer.readUInt32LE(60) : -1;
    if (buffer.toString('ascii', 0, 2) !== 'MZ' || offset < 64 || offset + 4 > buffer.length ||
        !buffer.subarray(offset, offset + 4).equals(Buffer.from([80, 69, 0, 0]))) throw new Error('Not a valid PE build');
    if (!/^[a-f0-9]{7,40}(?:-dirty)?$/.test(commit)) throw new Error('Invalid commit');
    const folder = `artifacts/${path.basename(file, '.exe')}`;
    const files = fs.existsSync(safe(folder)) ? fs.readdirSync(safe(folder)).map(f => `${folder}/${f}`) : [];
    files.forEach(evidence);
    state.builds.push({ ...fingerprint(file), commit, builtAt: new Date().toISOString(), accepted: false,
      evidence: files.map(fingerprint) });
    save(state);
  }
  function accept(file) {
    const state = read();
    const current = state.builds.at(-1);
    if (!current || current.file !== file) throw new Error('Only the current build can be accepted');
    verified(current);
    current.accepted = true;
    save(state);
  }
  function attachEvidence(file) {
    const state = read();
    if (!keep(state).has(file)) throw new Error('Evidence can only attach to current/rollback build');
    const build = state.builds.find(b => b.file === file);
    verified(build);
    build.evidence.forEach(e => verified(e));
    const folder = `artifacts/${path.basename(file, '.exe')}`;
    const existing = new Set(build.evidence.map(e => e.file.toLowerCase()));
    const files = fs.readdirSync(safe(folder)).filter(f => /\.(png|csv|html)$/.test(f)).map(f => `${folder}/${f}`);
    for (const f of files) {
      evidence(f);
      if (!existing.has(f.toLowerCase())) build.evidence.push(fingerprint(f));
    }
    save(state);
  }
  function bootstrap(config) {
    if (fs.existsSync(safe(manifest))) throw new Error('Already initialized');
    const state = { version: 1, builds: [], retired: [] };
    for (const b of config.builds) {
      executable(b.file);
      const info = fingerprint(b.file);
      if (b.sha256 && info.sha256 !== b.sha256) throw new Error(`Unexpected build: ${b.file}`);
      state.builds.push({ ...info, commit: b.commit ?? 'legacy', accepted: b.accepted ?? false,
        evidence: (b.evidence ?? []).map(f => { evidence(f); return fingerprint(f); }) });
    }
    state.retired = config.retired.map(f => { evidence(f); return fingerprint(f); });
    save(state);
  }
  function migrateEvidence() {
    const state = read();
    const retained = keep(state);
    const moves = state.builds.filter(b => retained.has(b.file)).flatMap(b => b.evidence
      .filter(e => e.file.startsWith('bin/')).map(e => ({ item: e,
        destination: `artifacts/${path.basename(b.file, '.exe')}/${path.basename(e.file)}` })));
    for (const m of moves) {
      verified(m.item);
      evidence(m.destination);
      if (fs.existsSync(safe(m.destination))) throw new Error(`Destination exists: ${m.destination}`);
    }
    // Copy and verify first, persist destination, then remove original. If an
    // interruption occurs, untracked leftovers are never swept automatically.
    for (const m of moves) {
      fs.mkdirSync(path.dirname(safe(m.destination)), { recursive: true });
      fs.copyFileSync(safe(m.item.file), safe(m.destination), fs.constants.COPYFILE_EXCL);
      verified({ ...m.item, file: m.destination });
    }
    const originals = moves.map(m => ({ ...m.item }));
    for (const m of moves) m.item.file = m.destination;
    save(state);
    for (const item of originals) { verified(item); fs.unlinkSync(safe(item.file)); }
    return moves.length;
  }
  return { safe, read, clean, audit, checkOutput, prepare, register, accept, attachEvidence, bootstrap, migrateEvidence, discard };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  let lock;
  let ownsLock = false;
  try {
    const args = process.argv.slice(2);
    const command = args[0] ?? 'plan';
    const apply = args.includes('--apply');
    const value = key => { const i = args.indexOf(key); if (i < 0 || !args[i + 1]) throw new Error(`Missing ${key}`); return args[i + 1]; };
    const m = manager(project);
    const writes = ['bootstrap', 'prepare', 'register', 'accept', 'attach-evidence', 'migrate-evidence', 'discard'];
    if (writes.includes(command) && !apply) throw new Error('Write command requires --apply');
    if (apply) {
      lock = m.safe('bin/artifacts.lock');
      fs.mkdirSync(path.dirname(lock), { recursive: true });
      fs.closeSync(fs.openSync(lock, 'wx'));
      ownsLock = true;
    }
    if (command === 'plan' || command === 'clean') console.log(JSON.stringify(m.clean(command === 'clean' && apply), null, 2));
    else if (command === 'audit') console.log(JSON.stringify(m.audit(args.includes('--allow-empty')), null, 2));
    else if (command === 'current') console.log(m.audit().current.file);
    else if (command === 'check-output') m.checkOutput(value('--file'));
    else if (command === 'bootstrap') m.bootstrap(JSON.parse(fs.readFileSync(path.join(project, 'scripts/artifacts-bootstrap.json'), 'utf8')));
    else if (command === 'prepare') console.log(m.prepare(value('--file')));
    else if (command === 'register') m.register(value('--file'), value('--commit'));
    else if (command === 'accept') m.accept(value('--file'));
    else if (command === 'attach-evidence') m.attachEvidence(value('--file'));
    else if (command === 'discard') m.discard(value('--file'));
    else if (command === 'migrate-evidence') console.log(`Migrated ${m.migrateEvidence()} evidence files`);
    else throw new Error('Unknown command');
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  } finally {
    if (ownsLock) fs.unlinkSync(lock);
  }
}
