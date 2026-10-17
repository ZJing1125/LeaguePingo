// Run with PLAYWRIGHT_MODULE pointing to an installed Playwright package and
// BROWSER_EXECUTABLE to a local Chromium/Edge. No downloads or live player data.
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const path = require('node:path')
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BROWSER_EXECUTABLE })
  try {
    const context = await browser.newContext({ viewport: { width: 540, height: 660 } })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    const bindings = await fs.readFile(path.join(__dirname, '../bindings/leaguepingo/appservice.ts'), 'utf8')
    const methods = Object.fromEntries([...bindings.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(match => [match[2], match[1]]))
    const stats = { games: 20, winRate: 55, kda: 3.1 }
    const snapshot = { connected: true, stage: 'in-game', phase: 'InProgress', frozen: true, message: '', participants: Array.from({ length: 10 }, (_, i) => ({ key: `test-${i}`, displayName: `测试玩家${i}#TEST`, team: i < 5 ? 'ally' : 'enemy', identityState: 'known', historyState: i < 5 ? 'ready' : 'pending', history: [], stats })) }
    let state = { revision: 0, visible: true, pinned: true, shortcutAvailable: true, warning: '' }
    const calls = []
    await page.route('http://127.0.0.1:43199/**', async route => {
      const url = new URL(route.request().url())
      if (url.pathname === '/wails/custom.js') return route.fulfill({ contentType: 'text/javascript', body: '' })
      if (url.pathname === '/wails/runtime') {
        const request = route.request().postDataJSON()
        const method = methods[request.args?.methodID]
        calls.push(method)
        let result = null
        if (method === 'Snapshot') result = snapshot
        else if (method === 'OverlayStatus') result = state
        else if (method === 'SetOverlayPinned') result = state = { ...state, revision: state.revision + 1, pinned: request.args.args[0] }
        else if (method === 'HideOverlay') result = state = { ...state, revision: state.revision + 1, visible: false }
        else if (method === 'RefreshRoster') result = snapshot
        else if (method === 'LookupPlayer') {
          const player = snapshot.participants.find(p => p.key === request.args.args[0])
          player.historyState = 'ready'
          result = player
        }
        else if (method === 'Lookup') return route.fulfill({ status: 400, json: { kind: 'RuntimeError', message: '玩家身份查询失败：LCU HTTP 400' } })
        return route.fulfill({ json: result })
      }
      const relative = url.pathname === '/' ? 'index.html' : url.pathname.slice(1)
      if (relative.includes('..')) return route.abort()
      const body = await fs.readFile(path.join(__dirname, '../dist', relative))
      const contentType = relative.endsWith('.js') ? 'text/javascript' : relative.endsWith('.css') ? 'text/css' : 'text/html'
      return route.fulfill({ body, contentType })
    })
    await page.goto('http://127.0.0.1:43199/?view=overlay')
    await page.getByRole('button', { name: '取消置顶', exact: true }).waitFor()
    assert.equal(await page.locator('.overlay-player').count(), 10)
    assert.equal(await page.locator('.overlay-head').evaluate(el => getComputedStyle(el).getPropertyValue('--wails-draggable').trim()), 'drag')
    assert.equal(await page.getByRole('button', { name: '关闭对局快览' }).evaluate(el => getComputedStyle(el).getPropertyValue('--wails-draggable').trim()), 'no-drag')
    const bounds = await page.locator('.overlay-footer').boundingBox()
    assert(bounds && bounds.y + bounds.height <= 660, 'footer/escape controls must stay within viewport')
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false)
    await page.getByRole('button', { name: '取消置顶', exact: true }).click()
    await page.getByRole('button', { name: '置顶', exact: true }).waitFor()
    assert.equal(state.pinned, false)
    assert.equal(await page.getByRole('button', { name: /穿透|恢复鼠标/ }).count(), 0)
    await page.getByRole('button', { name: '关闭对局快览' }).click()
    await page.waitForTimeout(100)
    assert.equal(state.visible, false)
    await page.keyboard.press('Escape')
    await page.waitForTimeout(100)
    assert(calls.filter(name => name === 'HideOverlay').length >= 2)
    state = { ...state, revision: state.revision + 1, warning: '快捷键注册失败，请使用关闭按钮或托盘' }
    await page.evaluate(state => window._wails.dispatchWailsEvent({ name: 'leaguepingo:overlay', data: state }), state)
    await page.getByText(state.warning).waitFor()
    if (process.env.OVERLAY_SCREENSHOT) await page.screenshot({ path: process.env.OVERLAY_SCREENSHOT })
    await page.setViewportSize({ width: 1100, height: 760 })
    await page.goto('http://127.0.0.1:43199/')
    await page.getByRole('button', { name: '隐藏快览', exact: true }).click()
    assert.equal(await page.getByRole('button', { name: /穿透|恢复鼠标/ }).count(), 0)
    await page.getByRole('button', { name: '刷新对局名单', exact: true }).click()
    await page.locator('.player-row').nth(5).click()
    await page.getByRole('button', { name: '手动查询此玩家', exact: true }).click()
    await page.waitForFunction(() => !document.querySelector('.detail-panel button'))
    assert(calls.includes('LookupPlayer'))
    assert(calls.includes('RefreshRoster'))
    await page.getByLabel('玩家完整 ID').fill('TestPlayer#TEST')
    await page.getByRole('button', { name: '查询战绩', exact: true }).click()
    await page.getByText('玩家身份查询失败：LCU HTTP 400', { exact: true }).waitFor()
    assert.equal(await page.getByText('RuntimeError:', { exact: false }).count(), 0)
    assert.deepEqual(errors, [])
    console.log('PASS: interactive overlay without click-through, drag/close/pin/Escape; in-game ID lookup, selected-enemy lookup, roster refresh and errors (mock bridge).')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
