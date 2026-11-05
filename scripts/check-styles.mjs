// Component ownership checks use locked Vue, PostCSS and selector parsing tools.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { classUsage, keyframeStep, positiveClasses, privateReferences, styleOwners } from './style-ownership.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const { parse } = require('@vue/compiler-sfc');
const postcss = require('postcss');

export function checkStyleOwnership(components, globalStyles) {
  const styles = new Map();
  for (const [file, source] of Object.entries(components)) {
    const { descriptor, errors } = parse(source, { filename: file });
    if (errors.length) throw new Error(`Invalid Vue component: ${file}`);
    if (descriptor.styles.some(style => !style.scoped && !style.module)) {
      throw new Error(`Component styles must be scoped or modules: ${file}`);
    }
    styles.set(file, descriptor.styles.map(style => style.content).join('\n'));
  }
  for (const owner of styleOwners.migratedComponents) {
    const file = [...styles.keys()].find(file => file.endsWith('/' + owner.component));
    const defined = new Set();
    if (file) postcss.parse(styles.get(file)).walkRules(rule => {
      if (!keyframeStep(rule)) for (const selector of rule.selectors) for (const name of positiveClasses(selector)) defined.add(name);
    });
    if (!file || !owner.privateClasses.some(name => defined.has(name))) {
      throw new Error(`Missing component-owned styles: ${owner.component}`);
    }
  }
  const uses = classUsage(components);
  for (const [file, source] of Object.entries(globalStyles)) {
    postcss.parse(source, { from: file }).walkRules(rule => {
      if (keyframeStep(rule)) return;
      for (const selector of rule.selectors) {
        for (const [name, owner] of privateReferences(selector, uses)) {
          throw new Error(`Private ${owner} selector .${name} returned to ${file}: ${selector}`);
        }
        for (const name of positiveClasses(selector)) {
          if (!uses.has(name) && !styleOwners.sharedUtilities.includes(name)) {
            throw new Error(`Unowned global selector .${name} in ${file}: ${selector}`);
          }
        }
      }
    });
  }
  return { components: styles.size, migratedFamilies: styleOwners.migratedComponents.length };
}

function componentFiles(folder) {
  return fs.readdirSync(folder, { withFileTypes: true }).flatMap(entry => {
    const file = path.join(folder, entry.name);
    if (entry.isDirectory()) return componentFiles(file);
    return entry.isFile() && entry.name.endsWith('.vue') ? [file] : [];
  });
}

function run() {
  const components = Object.fromEntries(componentFiles(path.join(root, 'frontend/src')).map(file => [path.relative(root, file).split(path.sep).join('/'), fs.readFileSync(file, 'utf8')]));
  const globalStyles = Object.fromEntries(fs.readdirSync(path.join(root, 'frontend/src')).filter(name => name.endsWith('.css')).map(name => ['frontend/src/' + name, fs.readFileSync(path.join(root, 'frontend/src', name), 'utf8')]));
  console.log(JSON.stringify({ ok: true, ...checkStyleOwnership(components, globalStyles) }));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { run(); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
