import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkStyleOwnership } from './check-styles.mjs';

const components = {
  'components/PlayerTabs.vue': '<template><div class="player-tabs" /></template><style scoped>.player-tabs{color:var(--text)}</style>',
  'components/SettingsPanel.vue': '<template><div class="settings-panel" /></template><style scoped>.settings-panel{background:var(--panel)}</style>',
  'components/RecentForm.vue': '<template><span class="recent-form" /></template><style scoped>.recent-form{display:inline-flex}</style>',
  'components/DesktopTitleBar.vue': '<template><header class="desktop-titlebar" /></template><style scoped>.window-controls{display:flex}</style>',
};
test('scoped ownership permits shared tokens, feedback, comments and declaration text', () => {
  assert.deepEqual(checkStyleOwnership(components, { 'shared.css': '/* .player-tabs{} */ :root{--caption:".settings-panel"}.settings-message{color:var(--text)}' }), { components: 4, migratedFamilies: 4 });
});
test('a component cannot reintroduce an unscoped style block', () => {
  const changed = { ...components, 'components/PlayerTabs.vue': components['components/PlayerTabs.vue'].replace('style scoped', 'style') };
  assert.throws(() => checkStyleOwnership(changed, {}), /must be scoped/);
});
test('private selectors cannot return to globals, including theme and media rules', () => {
  for (const selector of ['.player-tab.active', '.tab-close', ':root[data-theme=dark] .settings-panel', '.diagnostic-table td', '.recent-form', '.form-result.unknown', '.home-form-last .recent-form', '.window-controls button:focus-visible', '.desktop-titlebar .window-control-error']) {
    assert.throws(() => checkStyleOwnership(components, { 'legacy.css': `@media(max-width:880px){${selector}{color:red}}` }), /returned to/);
  }
});
test('ownership cannot be satisfied by deleting the migrated component styles', () => {
  assert.throws(() => checkStyleOwnership({ ...components, 'components/PlayerTabs.vue': '<template><div/></template>' }, {}), /Missing component-owned/);
});
