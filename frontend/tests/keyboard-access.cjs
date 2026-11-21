// Exercise real keyboard navigation against the existing synthetic application.
const assert = require('node:assert/strict');

function luminance(color) {
  if (/^#[0-9a-f]{3}$/i.test(color)) color = '#' + [...color.slice(1)].map(value => value + value).join('');
  const rgb = color.startsWith('#')
    ? color.slice(1).match(/../g).map(value => parseInt(value, 16))
    : color.match(/[\d.]+/g).slice(0, 3).map(Number);
  return rgb.map(value => value / 255).map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4)
    .reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0);
}
function contrast(first, second) {
  const values = [luminance(first), luminance(second)].sort((a, b) => a - b);
  return (values[1] + .05) / (values[0] + .05);
}

async function verifyKeyboardAccess(page, player, calls, capture = async () => {}) {
  // No click/focus/fill/selectOption: every user action below is a key press.
  async function tabTo(target) {
    await target.waitFor();
    for (let attempt = 0; attempt < 100; attempt++) {
      if (await target.evaluate(element => element === document.activeElement)) return;
      await page.keyboard.press('Tab');
    }
    assert.fail(`Keyboard could not reach ${await target.evaluate(element => element.outerHTML.slice(0, 160))}`);
  }
  async function visibleFocus(target) {
    const focus = await target.evaluate(element => {
      const style = getComputedStyle(element);
      let surface = element;
      while (surface.parentElement && getComputedStyle(surface).backgroundColor === 'rgba(0, 0, 0, 0)') surface = surface.parentElement;
      return {visible: element.matches(':focus-visible'), width: parseFloat(style.outlineWidth), kind: style.outlineStyle,
        color: style.outlineColor, background: getComputedStyle(surface).backgroundColor, offset: parseFloat(style.outlineOffset)};
    });
    assert(focus.visible && focus.width >= 2 && focus.kind !== 'none', `Missing keyboard focus: ${JSON.stringify(focus)}`);
    assert(contrast(focus.color, focus.background) >= 3, `Focus contrast below 3:1: ${JSON.stringify(focus)}`);
    return focus;
  }
  const nav = page.getByRole('navigation', {name: '工作区'});
  const home = page.locator('.home-workspace');
  const theme = page.getByRole('combobox', {name: '界面主题', exact: true});
  await page.emulateMedia({reducedMotion: 'reduce'});
  for (const selectedTheme of ['light', 'dark']) {
    await tabTo(theme);
    await page.keyboard.press('Home');
    if (selectedTheme === 'dark') await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    await page.waitForFunction(value => document.documentElement.dataset.theme === value, selectedTheme);
    const colors = await page.evaluate(() => {
      const style = getComputedStyle(document.documentElement);
      return Object.fromEntries(['text','surface','muted','panel','accent','win','win-bg','loss','loss-bg','on-accent','primary-bg']
        .map(name => [name, style.getPropertyValue(`--${name}`).trim()]));
    });
    for (const [foreground, background] of [['text','surface'],['muted','panel'],['accent','panel'],['win','win-bg'],['loss','loss-bg'],['on-accent','primary-bg']]) {
      assert(contrast(colors[foreground], colors[background]) >= 4.5, `${selectedTheme} ${foreground}/${background} text contrast below 4.5:1`);
    }
    await tabTo(nav.getByRole('button', {name: '首页', exact: true}));
    await page.keyboard.press('Enter');
    const rank = home.locator('.rank-history summary').first();
    await tabTo(rank); await visibleFocus(rank);
    await page.keyboard.press('Space');
    assert(await rank.evaluate(element => element.parentElement.open), 'Space did not open rank history');
    await page.keyboard.press('Space');
    const row = home.locator('.home-match .match-main').first();
    await tabTo(row);
    const rowFocus = await visibleFocus(row);
    assert(rowFocus.offset < 0, `row focus must fit inside its clipping container: ${JSON.stringify(rowFocus)}`);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', {name: '对局详情', exact: true});
    await dialog.waitFor(); await dialog.locator('tr.target').waitFor();
    await page.keyboard.press('Shift+Tab');
    assert(await dialog.evaluate(element => element.contains(document.activeElement)), 'reverse Tab escaped match dialog');
    await page.keyboard.press('Tab');
    assert(await dialog.evaluate(element => element.contains(document.activeElement)), 'Tab escaped match dialog');
    const firstTab = dialog.getByRole('tab', {name: '对局概览', exact: true});
    await tabTo(firstTab); await visibleFocus(firstTab);
    const timelineReads = calls.filter(name => name === 'MatchTimeline').length;
    await page.keyboard.press('End');
    assert.equal(await dialog.getByRole('tab', {name: '出装符文', exact: true}).getAttribute('aria-selected'), 'true');
    await page.keyboard.press('Home');
    assert(calls.filter(name => name === 'MatchTimeline').length <= timelineReads + 1, 'keyboard tab duplicated timeline request');
    const scoreboard = dialog.locator('.detail-table-scroll');
    await tabTo(scoreboard); await visibleFocus(scoreboard);
    await capture(`keyboard-${selectedTheme}-scoreboard`);
    await page.keyboard.press('Escape'); await dialog.waitFor({state: 'detached'});
    assert(await row.evaluate(element => element === document.activeElement), 'Escape failed to restore match-row focus');
    await visibleFocus(row);

    const search = page.getByRole('textbox', {name: '玩家完整 ID', exact: true});
    await tabTo(search); await visibleFocus(search);
    await page.keyboard.press('Control+A'); await page.keyboard.insertText(player.displayName); await page.keyboard.press('Enter');
    const history = page.locator('.players-workspace'); await history.locator('.match-main').first().waitFor();
    const queue = history.getByRole('combobox', {name: '战绩模式筛选', exact: true});
    await tabTo(queue); await visibleFocus(queue);
    await page.keyboard.press('Home'); await page.keyboard.press('Enter');
    const historyRow = history.locator('.match-main').first();
    await tabTo(historyRow); await page.keyboard.press('Space');
    await history.locator('.full-match-detail tr.target').waitFor();
    await tabTo(history.getByRole('tab', {name: '对局概览', exact: true}));
    await page.keyboard.press('ArrowRight');
    assert.equal(await history.getByRole('tab', {name: '团队分析', exact: true}).getAttribute('aria-selected'), 'true');
    await page.keyboard.press('ArrowLeft');
    await tabTo(historyRow); await page.keyboard.press('Space');

    const settingsButton = page.getByRole('button', {name: '设置与诊断', exact: true});
    await tabTo(settingsButton); await page.keyboard.press('Enter');
    const settings = page.getByRole('dialog', {name: '设置与诊断', exact: true}); await settings.waitFor();
    await page.keyboard.press('Tab'); await page.keyboard.press('Shift+Tab');
    assert(await settings.evaluate(element => element.contains(document.activeElement)), 'settings focus escaped');
    await tabTo(settings.getByRole('tab', {name: '外观', exact: true}));
    await page.keyboard.press('End');
    assert.equal(await settings.getByRole('tab', {name: '诊断与关于', exact: true}).getAttribute('aria-selected'), 'true');
    await page.keyboard.press('Escape'); await settings.waitFor({state: 'detached'});
    assert(await settingsButton.evaluate(element => element === document.activeElement), 'settings lost opener focus');
    await tabTo(nav.getByRole('button', {name: '对局室', exact: true})); await page.keyboard.press('Enter');
    assert.equal(await nav.getByRole('button', {name: '对局室', exact: true}).getAttribute('aria-current'), 'page');
    await tabTo(nav.getByRole('button', {name: '游戏辅助', exact: true})); await page.keyboard.press('Enter');
    const automation = page.getByRole('region', {name: '游戏辅助', exact: true});
    await tabTo(automation.getByRole('checkbox', {name: '启用自动接受对局', exact: true}));
    await visibleFocus(automation.getByRole('checkbox', {name: '启用自动接受对局', exact: true}));
    for (const name of ['启用自动接受对局','自动跳过点赞','赛后自动返回房间','返回后自动排队','选人阶段自动发送玩家分析']) {
      assert.equal(await automation.getByRole('checkbox',{name,exact:true}).isChecked(), false, 'navigation enabled automation');
    }
    assert.equal(await automation.getByRole('checkbox',{name:'包含自己',exact:true}).isChecked(),true,'report inclusion default changed');
    await tabTo(nav.getByRole('button', {name: '首页', exact: true})); await page.keyboard.press('Enter');
    assert.equal(await home.locator('.secondary').first().evaluate(element => getComputedStyle(element).transitionDuration), '0s', 'reduced motion was lost');
  }
}
module.exports = {verifyKeyboardAccess};
