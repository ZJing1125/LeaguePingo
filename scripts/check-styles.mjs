// Component ownership checks use the existing locked Vue/PostCSS toolchain.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const { parse } = require('@vue/compiler-sfc');
const postcss = require('postcss');

// Shared success feedback (.settings-message) is used by AutomationView too.
// Only these migrated, private families must stay out of global stylesheets.
const owners = [
  { component: 'PlayerTabs.vue', privateClass: /\.(?:player-tabs|player-tab|tab-close)(?![\w-])/ },
  { component: 'SettingsPanel.vue', privateClass: /\.(?:settings-panel|settings-backdrop|settings-shortcut|settings-actions|settings-hold|runtime-summary|diagnostic-table)(?![\w-])/ },
  { component: 'RecentForm.vue', privateClass: /\.(?:recent-form|form-result)(?![\w-])/ },
  { component: 'DesktopTitleBar.vue', privateClass: /\.(?:window-controls|window-control-error)(?![\w-])/ },
  { component: 'AppIcon.vue', privateClass: /\.app-icon(?![\w-])/ },
  { component: 'BrandIcon.vue', privateClass: /\.brand-mark(?![\w-])/ },
  { component: 'GameIcon.vue', privateClass: /\.game-icon(?![\w-])/, sharedReferences: ['.sidebar-hero>span:not(.game-icon)'] },
  { component: 'RankEmblem.vue', privateClass: /\.(?:rank-emblem|rank-placeholder)(?![\w-])/, sharedReferences: ['.sidebar-identity>span:not(.participant-avatar):not(.rank-emblem)'] },
  { component: 'ChampionPortrait.vue', privateClass: /\.(?:hero-art|portrait-fallback)(?![\w-])/ },
  { component: 'ChampionSplash.vue', privateClass: /\.(?:hero-splash|splash-fallback)(?![\w-])/ },
  { component: 'RoleIcon.vue', privateClass: /\.role-icon(?![\w-])/ },
  { component: 'QueueSwitch.vue', privateClass: /\.queue-switch(?![\w-])/ },
  { component: 'ScoutRing.vue', privateClass: /\.(?:scout-ring-metric|scout-ring|scout-ring-center)(?![\w-])/ },
];

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
  for (const owner of owners) {
    const file = [...styles.keys()].find(file => file.endsWith('/' + owner.component));
    if (!file || !owner.privateClass.test(styles.get(file))) {
      throw new Error(`Missing component-owned styles: ${owner.component}`);
    }
  }
  for (const [file, source] of Object.entries(globalStyles)) {
    postcss.parse(source, { from: file }).walkRules(rule => {
      for (const owner of owners) {
        // These exact :not exclusions style a sidebar sibling, not the icon.
        // Check each selector so a grouped private rule cannot hide behind one.
        for (const selector of rule.selectors) {
          if (owner.privateClass.test(selector) && !owner.sharedReferences?.includes(selector)) {
            throw new Error(`Private ${owner.component} selector returned to ${file}: ${selector}`);
          }
        }
      }
    });
  }
  return { components: styles.size, migratedFamilies: owners.length };
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
