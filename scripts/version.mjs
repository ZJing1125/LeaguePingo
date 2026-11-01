import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const semver = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

export function parseVersion(value) {
  const match = semver.exec(value.trim());
  if (!match) throw new Error('VERSION must be a SemVer value');
  if ((match[4] ?? '').split('.').some(part => /^\d+$/.test(part) && part.length > 1 && part.startsWith('0'))) {
    throw new Error('VERSION must not contain a numeric prerelease identifier with a leading zero');
  }
  return { value: value.trim(), base: `${match[1]}.${match[2]}.${match[3]}`, numeric: `${match[1]}.${match[2]}.${match[3]}.0` };
}

export function checkVersion(read = file => fs.readFileSync(path.join(root, file), 'utf8')) {
  const version = parseVersion(read('VERSION'));
  const required = [
    ['preferences.go', `var Version = "${version.value}"`],
    ['build/config.yml', `version: "${version.base}"`],
    ['build/windows/info.json', `"file_version": "${version.numeric}"`],
    ['build/windows/info.json', `"ProductVersion": "${version.value}"`],
    ['build/windows/wails.exe.manifest', `version="${version.numeric}"`],
    ['build/windows/nsis/wails_tools.nsh', `!define INFO_PRODUCTVERSION "${version.value}"`],
    ['build/windows/nsis/wails_tools.nsh', `!define INFO_NUMERICVERSION "${version.numeric}"`],
  ];
  for (const [file, text] of required) {
    if (!read(file).includes(text)) throw new Error(`Version metadata mismatch: ${file}`);
  }
  const manifest = JSON.parse(read('update.json'));
  parseVersion(manifest.version);
  if (manifest.schemaVersion !== 1 || manifest.url !== 'https://github.com/ZJing1125/LeaguePingo/releases/latest') {
    throw new Error('Invalid static update manifest');
  }
  return version;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const command = process.argv[2] ?? 'check';
    const version = checkVersion();
    if (command === 'check') console.log(JSON.stringify(version));
    else if (['value', 'base', 'numeric'].includes(command)) console.log(version[command]);
    else throw new Error('Usage: node scripts/version.mjs [check|value|base|numeric]');
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
