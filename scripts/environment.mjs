import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Read-only discovery. Never install tools, download a browser or print arbitrary
// environment variables (which can contain credentials).
export function findTool(name, env = process.env, platform = process.platform, exists = fs.existsSync) {
  const separator = platform === 'win32' ? ';' : ':';
  const extensions = platform === 'win32' ? ['', '.exe', '.cmd', '.bat'] : [''];
  for (const folder of (env.PATH || env.Path || '').split(separator).filter(Boolean)) {
    for (const ext of extensions) {
      const candidate = path.join(folder.replace(/^"|"$/g, ''), name + ext);
      if (exists(candidate)) return candidate;
    }
  }
  return null;
}

export function findBrowser(env = process.env, platform = process.platform, exists = fs.existsSync) {
  if (env.BROWSER_EXECUTABLE) {
    if (!exists(env.BROWSER_EXECUTABLE)) throw new Error('Configured BROWSER_EXECUTABLE does not exist');
    return env.BROWSER_EXECUTABLE;
  }
  if (platform === 'win32') {
    for (const folder of [env['ProgramFiles(x86)'], env.ProgramFiles, env.LOCALAPPDATA].filter(Boolean)) {
      for (const relative of ['Microsoft/Edge/Application/msedge.exe', 'Google/Chrome/Application/chrome.exe']) {
        const candidate = path.join(folder, relative);
        if (exists(candidate)) return candidate;
      }
    }
  }
  for (const name of ['chromium', 'chromium-browser', 'google-chrome', 'msedge']) {
    const candidate = findTool(name, env, platform, exists);
    if (candidate) return candidate;
  }
  throw new Error('No local browser found; configure BROWSER_EXECUTABLE (no automatic download)');
}

export function findNSIS(env = process.env, platform = process.platform, exists = fs.existsSync) {
  const configured = env.LEAGUEPINGO_MAKENSIS;
  if (configured) {
    if (!exists(configured)) throw new Error('Configured LEAGUEPINGO_MAKENSIS does not exist');
    return configured;
  }
  const found = findTool('makensis', env, platform, exists);
  if (found) return found;
  if (platform === 'win32') {
    for (const folder of [env['ProgramFiles(x86)'], env.ProgramFiles].filter(Boolean)) {
      const candidate = path.join(folder, 'NSIS', 'makensis.exe');
      if (exists(candidate)) return candidate;
    }
  }
  throw new Error('NSIS not found; install NSIS or configure LEAGUEPINGO_MAKENSIS');
}

export function describeEnvironment(env = process.env, platform = process.platform, arch = process.arch) {
  const tools = Object.fromEntries(['bash', 'go', 'node', 'npm', 'wails3'].map(n => [n, Boolean(findTool(n, env, platform))]));
  let browser = false;
  try { browser = Boolean(findBrowser(env, platform)); } catch { /* Report unavailable, not successful verification. */ }
  let nsis = false;
  try { nsis = Boolean(findNSIS(env, platform)); } catch { /* Optional packaging dependency. */ }
  return { hostPlatform: platform, hostArchitecture: arch, tools, browserAvailable: browser,
    nsisAvailable: nsis,
    portableToolsPresent: tools.bash && tools.go && tools.node && tools.npm,
    windowsDeliveryHost: platform === 'win32' && arch === 'x64',
    note: 'Discovery only; dependencies, tool versions and tests have not been validated. No remote machine was inspected.' };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.argv[2] === 'browser') console.log(findBrowser());
    else if (process.argv[2] === 'nsis') console.log(findNSIS());
    else if (!process.argv[2] || process.argv[2] === 'doctor') console.log(JSON.stringify(describeEnvironment(), null, 2));
    else throw new Error('Usage: node scripts/environment.mjs [doctor|browser|nsis]');
  } catch (e) { console.error(e.message); process.exitCode = 1; }
}
