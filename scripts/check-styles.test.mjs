import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkStyleOwnership } from './check-styles.mjs';
import { styleOwners, templateClasses } from './style-ownership.mjs';

// Every migrated family is present; mutations below exercise policy decisions.
const components = Object.fromEntries(styleOwners.migratedComponents.map(owner => [
  `components/${owner.component}`,
  `<template><div class="${owner.privateClasses.join(' ')}" /></template><style scoped>.${owner.privateClasses[0]}{color:var(--text)}</style>`,
]));
for (const name of ['SharedA', 'SharedB']) components[`components/${name}.vue`] = '<template><div class="settings-message shared-control next-private-caption"/></template>';

test('scoped ownership permits tokens, shared controls, negative references and keyframes', () => {
  const styles = '/* .player-tabs{} */ :root{--caption:".settings-panel"}.settings-message{color:var(--text)}span:not(.game-icon){flex:1}:is(.desktop-app,.drawer-floating,.overlay-shell) button{outline:0}@keyframes pulse{from{opacity:0}to{opacity:1}}';
  assert.deepEqual(checkStyleOwnership(components, { 'shared.css': styles }), { components: Object.keys(components).length, migratedFamilies: styleOwners.migratedComponents.length });
});

test('root and nested components cannot reintroduce unscoped styles', () => {
  for (const file of ['components/App.vue', 'components/PlayerTabs.vue']) {
    assert.throws(() => checkStyleOwnership({ ...components, [file]: components[file].replace('style scoped', 'style') }, {}), /must be scoped/);
  }
  checkStyleOwnership({ ...components, 'components/PlayerTabs.vue': components['components/PlayerTabs.vue'].replace('style scoped', 'style module') }, {});
});

test('private selectors cannot return through grouping, pseudo functions, escapes, theme or media', () => {
  for (const selector of ['.player-tab.active', '.tab-close', ':root[data-theme=dark] .settings-panel', '.diagnostic-table td', '.recent-form', '.form-result.unknown', '.home-form-last .recent-form', '.window-controls button:focus-visible', '.desktop-titlebar .window-control-error', '.app-icon', '.sidebar-header .brand-mark', '.game-icon', '.rank-placeholder', '.card-portrait .hero-art', '.home-hero-art .hero-splash', '.scout-ring .role-icon', '.queue-switch button[aria-pressed=true]', '.scout-ring-metric > small', '.sidebar-hero>span:not(.game-icon)', 'span:not(.game-icon),.game-icon', ':is(.overlay-shell) button', ':is(.match-main,.timeline-events):focus-visible', '.\\70 layer-tabs']) {
    assert.throws(() => checkStyleOwnership(components, { 'legacy.css': `@media(max-width:880px){${selector}{color:red}}` }), /returned to/);
  }
});

test('deleting or replacing owned styles with comments fails for every migrated component', () => {
  for (const [file, source] of Object.entries(components).filter(([, source]) => source.includes('<style scoped>'))) {
    assert.throws(() => checkStyleOwnership({ ...components, [file]: '<template><div/></template>' }, {}), /Missing component-owned/);
    const style = source.slice(source.indexOf('<style scoped>') + 14, source.indexOf('</style>'));
    assert.throws(() => checkStyleOwnership({ ...components, [file]: `<template><div/></template><style scoped>/* ${style} */</style>` }, {}), /Missing component-owned/);
  }
});

test('a new component is protected without adding another owner regex', () => {
  const next = { ...components, 'components/NewPanel.vue': '<template><div class="next-private"/></template><style scoped>.next-private{display:block}</style>' };
  assert.throws(() => checkStyleOwnership(next, { 'shared.css': '.next-private{display:none}' }), /NewPanel.vue/);
  checkStyleOwnership(next, { 'shared.css': '.next-private-caption{display:block}' });
});

test('unreferenced globals cannot accumulate as legacy CSS again', () => {
  assert.throws(() => checkStyleOwnership(components, { 'legacy.css': '.unused-old-panel{display:none}' }), /Unowned global/);
  checkStyleOwnership(components, { 'shared.css': '.shared-control{display:flex}.sr-only{position:absolute}' });
});

test('class producers include bindings, class props and transition states', () => {
  const source = '<template><Transition name="notice"><div class="base" :class="{ compact, selected: active, \'custom-state\': true }"><Child banner-class="passed-class"/></div></Transition></template>';
  const names = templateClasses(source, 'Sample.vue');
  for (const name of ['base', 'compact', 'selected', 'custom-state', 'passed-class', 'notice-enter-active', 'notice-leave-to']) assert(names.has(name), name);
});
