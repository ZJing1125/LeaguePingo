// Synthetic bridge: account ownership, bounded home preparation and theme choice.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const { mockWindowRuntime, normalizeServiceRequest } = require("./window-runtime.cjs");
const { saveUIArtifact } = require('./ui-artifacts.cjs');

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BROWSER_EXECUTABLE });
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: Number(process.env.UI_DEVICE_SCALE_FACTOR || 1) });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [], external = [], calls = [], backgroundCalls = [];
    page.on("pageerror", e => errors.push(e.message));
    const desktop = mockWindowRuntime(page);
    const binding = await fs.readFile(path.join(__dirname, "../bindings/leaguepingo/appservice.ts"), "utf8");
    const methods = Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m => [m[2], m[1]]));
    const avatar = "data:image/svg+xml;base64," + Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect width="64" height="64" fill="#6652a8"/><circle cx="32" cy="32" r="18" fill="#efc280"/></svg>').toString("base64");
    function player(key = "synthetic-self", connectionId = 1) {
      return {
        key, puuid: key, connectionId, displayName: key === "synthetic-self" ? "我的合成账号#DEMO" : "查询的其他玩家#DEMO", identityState: "known", historyState: "ready", privacyState: "public", team: "ally", championId: 107,
        history: Array.from({ length: 20 }, (_, i) => ({ gameId: 50000 + i, playedAt: "2026-10-20T03:00:00Z", queueId: i % 3 ? 420 : 440, championId: [107, 23, 25, 99, 17][i % 5], win: i % 4 !== 0, kills: 6 + i % 3, deaths: 3, assists: 8, duration: 1810, cs: 180+i, damage: 24000+i*1000, gold: 12345+i*100, runePrimary:8000,runeSecondary:8100,runeIds:[8005,9111,9104,8014,8139,8135],items: [3006,3078,3031,3153,0,0,3364], spell1: 4, spell2: 14 })),
        historyMeta: { fetchedAt: "2026-10-20T03:00:00Z", returned: 20, parsed: 20 }, stats: { games: 20 },
        profile: { updatedAt: new Date().toISOString(), level: 128, profileIconData: avatar, privacyState: "public", rankState: "ready", ranks: [{ queue: "RANKED_SOLO_5x5", tier: "EMERALD", division: "II", lp: 48, wins: 65, losses: 47 }, { queue: "RANKED_FLEX_SR", tier: "GOLD", division: "I", lp: 23, wins: 25, losses: 21 }], fields: { rank: { state: "ready", fetchedAt: "2026-10-20T03:00:00Z" } } },
      };
    }
    let queueRelease,queueEntered;
    let self = player(), failHome = false, deferHome = false, releaseHome, homeEntered;
    let snapshot = { connectionId: 1, selfKey: self.key, generation: 1, connected: true, phase: "None", stage: "idle", frozen: false, updatedAt: "2026-10-20T03:00:00Z", participants: [] };
    await page.route("**/*", route => { external.push(route.request().url()); return route.abort(); });
    await page.route("http://127.0.0.1:43202/**", async route => {
      const url = new URL(route.request().url());
      if (url.pathname === "/wails/custom.js") return route.fulfill({ contentType: "text/javascript", body: "" });
      if (url.pathname === "/wails/runtime") {
        const request = normalizeServiceRequest(route.request().postDataJSON());
        if (await desktop.handle(route, request)) return;
        const method = methods[request.args?.methodID];
        const args = request.args?.args ?? [];
        // Ranked warm-up is independent of foreground/theme actions. Provide
        // a bounded exhausted page rather than depending on finishing <1.2s.
        if (method === 'HistoryPage') {
          backgroundCalls.push(args);
          assert.equal(args[2], 20, 'empty warm-up page started another scan');
          return route.fulfill({ json: { matches: [], meta: { returned: 0, parsed: 0, excluded: 0, fetchedAt: '2026-10-20T03:00:00Z' }, offset: 20, nextOffset: 20, hasMore: false } });
        }
        calls.push(method);
        let result;
        if (method === 'GuideStatus') result={revision:0,status:'idle',query:{},base:null,matchup:null};
    else if (method === 'PostGameStatus') result={revision:0,returnEnabled:false,queueEnabled:false,delaySeconds:10,status:'off',dueAt:'0001-01-01T00:00:00Z'};
        else if (method === "AutoAcceptStatus") result = {revision:0,enabled:false,delaySeconds:1,status:"off",dueAt:"0001-01-01T00:00:00Z"};
        else if (method === "Snapshot") result = snapshot;
        else if (method === "OverlayStatus") result = { visible: false, pinned: true, revision: 0 };
        else if (method === "CurrentPlayer") {
          if (deferHome) await new Promise(resolve => { releaseHome = resolve; homeEntered?.(); });
          if (failHome) return route.fulfill({ status: 500, json: { message: "当前账号资料暂不可用" } });
          result = self;
        } else if (method === "VerifyHistoryQueue") {queueEntered?.();await new Promise(resolve=>{queueRelease=resolve;});result={queueId:2400,state:"verified",compared:30,mixed:60,filtered:40};}
        else if (method === "EnsureProfile") result = args[0] === self.key ? self : player(args[0], snapshot.connectionId);
        else if (method === "Lookup") result = player("synthetic-other", snapshot.connectionId);
        else if (method === "RefreshData") { self = { ...self, profile: { ...self.profile, updatedAt: new Date(Date.now() + 1000).toISOString(), level: 129 } }; result = self; }
        else if (method === "MatchDetail") {
          const target = self.history.find(match => match.gameId === args[2]) ?? self.history[0];
          result = { gameId: args[2], connectionId: snapshot.connectionId, partial: false, fetchedAt: "2026-10-20T03:04:00Z", players: Array.from({ length: 10 }, (_, i) => ({ participantId: i + 1, teamId: i < 5 ? 100 : 200, displayName: `详情玩家${i + 1}#DEMO`, target: i === 0, summary: { ...target, championId: [107,23,25,99,17,64,103,222,412,111][i], kills: 5 + i % 4, deaths: 2 + i % 3, assists: 7 + i % 5, participation: undefined, position: ["TOP","JUNGLE","MIDDLE","BOTTOM","UTILITY"][i % 5], visionScore: 10 + i, wardsPlaced: 4 + i, wardsKilled: i % 4, controlWards: i % 3, damageTaken: 15000 + i * 600, championLevel: 15 + i % 3 }, participation: 40 + i })) };
        }
        else if (method === "Info") result = { version: "synthetic", commit: "test", requests: {}, shortcut: "Ctrl+Shift+P", holdEnabled: true, holdKey: "Backquote", goHeapBytes: 0, goroutines: 0 };
        else throw new Error(`Unexpected service: ${method}`);
        return route.fulfill({ json: result });
      }
      const relative = url.pathname === "/" ? "index.html" : url.pathname.slice(1);
      if (relative.includes("..")) return route.abort();
      return route.fulfill({ body: await fs.readFile(path.join(__dirname, "../dist", relative)), contentType: relative.endsWith(".js") ? "text/javascript" : relative.endsWith(".css") ? "text/css" : relative.endsWith(".jpg") ? "image/jpeg" : relative.endsWith(".png") ? "image/png" : "text/html" });
    });
    const nav = page.getByRole("navigation", { name: "工作区" });
    const theme = page.getByRole("combobox", { name: "界面主题", exact: true });
    async function emit() { snapshot.updatedAt = new Date().toISOString(); await page.evaluate(data => window._wails.dispatchWailsEvent({ name: "leaguepingo:snapshot", data }), snapshot); }
    async function resolved(value) { await page.waitForFunction(v => document.documentElement.dataset.theme === v, value); }
    async function screenshot(name) {
      await saveUIArtifact(page,process.env.UI_SCREENSHOT_PREFIX,name,name.startsWith('single-game'));
    }
    await page.goto("http://127.0.0.1:43202/");
    await page.locator(".home-account h2").filter({ hasText: self.displayName }).waitFor();
    const connectionGuide = page.getByRole('region', { name: '连接游戏客户端', exact: true });
    assert.equal(await connectionGuide.count(), 0, 'connected home must not show connection instructions');
    assert.equal(await nav.getByRole("button", { name: "首页", exact: true }).getAttribute("aria-current"), "page");
    assert.equal(await page.locator(".home-match").count(), 20);
    const homeSidebar = page.locator(".home-ranks");
    assert.equal(await homeSidebar.locator(".champion-table tbody tr").count(), 5);
    assert.equal(await page.locator(".home-analysis-main .champion-analysis").count(), 0, "common champions still consumed match-column height");
    assert(await homeSidebar.evaluate(sidebar => { const champions=sidebar.querySelector('.champion-analysis'),mastery=sidebar.querySelector('.mastery-panel'); return !!champions && !!mastery && !!(champions.compareDocumentPosition(mastery) & Node.DOCUMENT_POSITION_FOLLOWING); }), "common champions must precede all-mode mastery");
    assert(await page.locator(".home-match-list").evaluate(el => getComputedStyle(el).overflowY === 'visible' && el.scrollHeight <= el.clientHeight + 1), "recent matches still used a nested fixed-height scroller");
    assert((await page.locator(".home-ranks").innerText()).includes("翡翠 II"));
    assert((await page.locator(".home-ranks").innerText()).includes("黄金 I"));
    assert.equal(calls.filter(c => c === "CurrentPlayer").length, 1);
    assert(await page.locator(".home-account img").evaluate(img => img.complete && img.naturalWidth > 0));
    await page.getByRole("img", { name: "玩家账号头像", exact: true }).waitFor();
    await resolved("light");
    assert.equal(await theme.inputValue(), "light", "a new player should start in light mode");
    const detailsBeforeSingleGame=calls.filter(call=>call==='MatchDetail').length;
    await page.locator('.home-match .match-main').first().click();
    const singleGame=page.getByRole('dialog',{name:'对局详情',exact:true});
    await singleGame.waitFor();
    await singleGame.locator('tr.target').waitFor();
    assert.equal(await nav.getByRole('button',{name:'首页',exact:true}).getAttribute('aria-current'),'page','single match left the home workspace');
    assert.equal(await singleGame.getByRole('region',{name:'双方对局记分板'}).count(),1);
    assert((await singleGame.locator('.analysis-tabs [role=tab]').first().getAttribute('id')).includes('50000'),'home detail opened a different game');
    assert.equal(await singleGame.locator('.detail-loadout').count(),10);
    assert.equal(calls.filter(call=>call==='MatchDetail').length,detailsBeforeSingleGame+1,'opening one home game must load one detail');
    for(const [width,height] of [[1280,800],[1100,760],[880,620]]){
      await page.setViewportSize({width,height});
      assert(await singleGame.evaluate(el=>el.scrollWidth<=el.clientWidth+1),'single-game dialog overflow');
      assert(await singleGame.locator('.detail-loadout').first().evaluate(el=>new Set([...el.children].map(child=>Math.round(child.getBoundingClientRect().top))).size===1),'seven equipment slots wrapped in the scoreboard');
      const fit=await singleGame.boundingBox();assert(fit.x>=0 && fit.y>=0 && fit.x+fit.width<=width+1 && fit.y+fit.height<=height+1,`single-game dialog escaped ${width}x${height}: ${JSON.stringify(fit)}`);
      await screenshot(`single-game-${width}`);
    }
    await page.evaluate(()=>{localStorage.setItem('leaguepingo.theme','light');dispatchEvent(new StorageEvent('storage',{key:'leaguepingo.theme',newValue:'light'}));});
    await screenshot('single-game-light');
    await page.evaluate(()=>{localStorage.setItem('leaguepingo.theme','dark');dispatchEvent(new StorageEvent('storage',{key:'leaguepingo.theme',newValue:'dark'}));});
    await page.setViewportSize({width:1280,height:800});
    await singleGame.press('Escape');
    await singleGame.waitFor({state:'detached'});
    assert.equal(await page.locator('.home-match .match-main').first().evaluate(el=>el===document.activeElement),true,'closing detail lost row focus');
    async function checkBrand() {
      const marks = await page.locator("img.brand-mark").evaluateAll(images => images.map(img => ({
        loaded: img.complete && img.naturalWidth > 0,
        source: new URL(img.currentSrc).pathname,
        clip: getComputedStyle(img).clipPath,
        background: getComputedStyle(img).backgroundColor,
        dpr: window.devicePixelRatio,
      })));
      assert.equal(marks.length, 1, "the unified toolbar must show the approved logo only once");
      assert.equal(await page.locator("header.desktop-titlebar.app-toolbar").count(), 1);
      for (const mark of marks) {
        assert(mark.loaded && /^\/brand\/icon-(32|64|96)\.png$/.test(mark.source), "brand asset must load locally");
        assert.equal(mark.source, `/brand/icon-${mark.dpr <= 1 ? 32 : mark.dpr <= 2 ? 64 : 96}.png`, "brand source must follow display density");
        assert.equal(mark.clip, "none", "the old letter P cutout must not crop the scout");
        assert.equal(mark.background, "rgba(0, 0, 0, 0)", "theme accent must not recolour the icon");
      }
    }
    await checkBrand();
    await theme.selectOption("light"); await resolved("light");
    await checkBrand();
    await page.reload();
    await page.locator(".home-account h2").filter({ hasText: self.displayName }).waitFor();
    await resolved("light");
    assert.equal(await theme.inputValue(), "light", "saved light mode must override the dark default");
    await screenshot("light");
    let before = calls.length;
    await theme.selectOption("dark"); await resolved("dark");
    await screenshot("dark");
    assert.equal(await page.locator("body").evaluate(el => getComputedStyle(el).backgroundColor), "rgb(28, 28, 31)");
    await page.emulateMedia({ colorScheme: "light" });
    await resolved("dark");
    assert.equal(calls.length, before, "theme requested player data");
    await page.reload();
    await page.locator(".home-account h2").filter({ hasText: self.displayName }).waitFor();
    await resolved("dark"); assert.equal(await theme.inputValue(), "dark");
    before = calls.length;
    await theme.selectOption("system"); await resolved("light");
    await page.emulateMedia({ colorScheme: "dark" }); await resolved("dark");
    await page.emulateMedia({ colorScheme: "light" }); await resolved("light");
    assert.equal(calls.length, before);
    await page.getByRole("button", { name: "设置与诊断", exact: true }).click();
    const settings = page.getByRole("dialog", { name: "设置与诊断", exact: true });
    await settings.getByRole("radio", { name: /深色/ }).check(); await resolved("dark");
    assert.equal(await theme.inputValue(), "dark");
    await screenshot("settings");
    await settings.getByRole('tab', { name: '外观', exact: true }).focus();
    await page.keyboard.press('ArrowRight');
    assert.equal(await settings.getByRole('tab', { name: '游戏辅助' }).getAttribute('aria-selected'), 'true');
    await page.keyboard.press('ArrowLeft');
    assert.equal(await settings.getByRole('tab', { name: '外观', exact: true }).getAttribute('aria-selected'), 'true');
    await settings.getByRole('tab',{name:'诊断与关于',exact:true}).click();
    const entered=new Promise(resolve=>{queueEntered=resolve;});
    await settings.getByRole('button',{name:'核验海克斯模式查询',exact:true}).click();
    await entered;
    assert(await settings.getByRole('button',{name:'正在核验，最多45秒…',exact:true}).isDisabled());
    queueRelease();
    await settings.getByText('已核对 30 场，本次连接已启用海克斯直查',{exact:true}).waitFor();
    await settings.press("Escape");
    assert(await page.getByRole("button", { name: "设置与诊断", exact: true }).evaluate(el => el === document.activeElement));
    before = calls.length;
    const peer = await page.context().newPage();
    await peer.route("http://127.0.0.1:43202/theme-sync", route => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Theme storage peer</title>" }));
    await peer.goto("http://127.0.0.1:43202/theme-sync");
    await peer.evaluate(() => localStorage.setItem("leaguepingo.theme", "light"));
    await resolved("light"); assert.equal(await theme.inputValue(), "light");
    await peer.evaluate(() => localStorage.removeItem("leaguepingo.theme"));
    await resolved("light"); assert.equal(await theme.inputValue(), "light");
    await peer.evaluate(() => localStorage.setItem("leaguepingo.theme", "light"));
    await resolved("light");
    await peer.close();
    assert.equal(calls.length, before, "cross-window theme sync requested data");
    await page.evaluate(() => { window.__themeSetItem = Storage.prototype.setItem; Storage.prototype.setItem = function () { throw new DOMException("Synthetic storage unavailable", "SecurityError"); }; });
    await theme.selectOption("dark"); await resolved("dark");
    await page.getByRole("button", { name: "设置与诊断", exact: true }).click();
    await settings.getByText("主题已应用，本次无法保存选择。", { exact: true }).waitFor();
    await settings.press("Escape");
    await page.evaluate(() => { Storage.prototype.setItem = window.__themeSetItem; delete window.__themeSetItem; });
    await page.locator(".home-heading button").click();
    await page.locator(".home-account").filter({ hasText: "等级 129" }).waitFor();
    assert.equal(calls.filter(c => c === "RefreshData").length, 1);
    await page.getByRole("textbox", { name: "玩家完整 ID" }).fill("其他账号#DEMO");
    try {
      await page.getByRole("button", { name: "查询战绩", exact: true }).click();
    } catch (error) {
      // This page has only synthetic data. Capture the state that can hide the
      // accessible button, without changing the selector, timeout or assertion.
      console.error('Synthetic home query failure state:', await page.evaluate(() => {
        const button = document.querySelector('.global-search button');
        return {
          toolbarInert: document.querySelector('.toolbar-content')?.hasAttribute('inert'),
          dialogs: document.querySelectorAll('[role="dialog"]').length,
          searchBusy: document.querySelector('.global-search')?.getAttribute('aria-busy'),
          button: button ? { text: button.textContent, disabled: button.hasAttribute('disabled'), inertAncestor: !!button.closest('[inert]'), rect: button.getBoundingClientRect().toJSON() } : null,
        };
      }));
      await screenshot('home-query-failure');
      throw error;
    }
    await page.locator(".players-workspace .identity-title").filter({ hasText: "查询的其他玩家#DEMO" }).waitFor();
    const beforeCollapse = calls.length;
    const originalWidth = await page.locator('.players-workspace .history-column').evaluate(el=>el.getBoundingClientRect().width);
    await page.getByRole('button', { name: '收起资料', exact: true }).click();
    assert(await page.locator('.players-workspace .history-column').evaluate(el=>el.getBoundingClientRect().width) > originalWidth);
    assert.equal(await page.locator('.players-workspace .analysis-sidebar').isVisible(), false);
    await page.getByRole('button', { name: '展开资料', exact: true }).click();
    assert.equal(calls.length, beforeCollapse, 'profile layout must not query player data');
    await screenshot("history-dark");
    await nav.getByRole("button", { name: "首页", exact: true }).click();
    assert.equal(await page.locator(".home-account h2").innerText(), "我的合成账号#DEMO");
    before = calls.length;
    snapshot.phase = "Lobby"; snapshot.stage = "party"; snapshot.generation++;
    snapshot.participants = [player("synthetic-other"), self];
    await emit();
    await nav.getByRole("button", { name: "对局室", exact: true }).click();
    await screenshot("roster-dark");
    await nav.getByRole("button", { name: "首页", exact: true }).click();
    await emit();
    assert.equal(calls.length, before, "cached home navigation or snapshot caused requests");
    for (const size of [{ width: 1100, height: 760 }, { width: 880, height: 620 }]) {
      await page.setViewportSize(size);
      assert(await page.locator(".app-toolbar").evaluate(el => el.scrollWidth <= el.clientWidth + 1), "toolbar overflow");
      assert(await page.locator(".toolbar-content").evaluate(el => el.scrollWidth <= el.clientWidth + 1), "toolbar content overflow");
      assert(await page.locator(".window-controls button").evaluateAll(buttons => buttons.every(button => {
        const rect = button.getBoundingClientRect();
        return rect.width > 0 && rect.left >= 0 && rect.right <= innerWidth && rect.top >= 0 && rect.bottom <= 64;
      })), "window controls must stay visible in the same top row");
      assert(await page.locator(".home-workspace").evaluate(el => el.scrollWidth <= el.clientWidth + 1), "home overflow");
      await screenshot(`dark-${size.width}`);
    }
    before = calls.length;
    const backgroundBeforeDisconnect = backgroundCalls.length;
    snapshot.connected = false; snapshot.generation++;
    await emit();
    await connectionGuide.waitFor();
    assert.deepEqual(await connectionGuide.getByRole('listitem').allTextContents(), [
      '1启动腾讯国服英雄联盟客户端',
      '2完成登录，进入客户端首页',
      '3保持 LeaguePingo 运行，等待自动连接',
    ]);
    assert.equal(await page.getByRole('dialog').count(), 0, 'connection instructions must not block cached browsing');
    assert((await page.locator(".home-account").innerText()).includes("旧资料"));
    for (const selectedTheme of ['dark', 'light']) {
      await theme.selectOption(selectedTheme); await resolved(selectedTheme);
      for (const width of [880, 1280]) {
        await page.setViewportSize({width, height: 800});
        assert(await connectionGuide.evaluate(el => el.scrollWidth <= el.clientWidth + 1), 'connection guide overflow');
        await screenshot(`connection-guide-${selectedTheme}-${width}`);
      }
    }
    assert.equal(calls.length, before, 'connection instructions requested data');
    assert.equal(backgroundCalls.length, backgroundBeforeDisconnect, 'connection instructions launched history scanning');
    // A new account cannot inherit the previous user's data; a failed load does
    // not restart on every snapshot, theme change or navigation.
    snapshot = { ...snapshot, connected: true, connectionId: 2, selfKey: "synthetic-new", phase: "None", stage: "idle", generation: snapshot.generation + 1, participants: [] };
    failHome = true;
    await emit();
    await page.locator(".home-error").waitFor();
    assert.equal(await connectionGuide.count(), 0, 'instructions must disappear on reconnect even if player data is unavailable');
    assert(!(await page.locator(".home-account").innerText()).includes("我的合成账号"));
    before = calls.filter(c => c === "CurrentPlayer").length;
    await emit(); await nav.getByRole("button", { name: "对局室", exact: true }).click(); await nav.getByRole("button", { name: "首页", exact: true }).click();
    assert.equal(calls.filter(c => c === "CurrentPlayer").length, before);
    failHome = false; self = player("synthetic-new", 2);
    await page.locator(".home-heading button").click();
    await page.locator(".home-account h2").filter({ hasText: self.displayName }).waitFor();
    // Missing rank and match outcomes remain unknown, rather than becoming zero
    // wins or an unranked claim. No automatic request is made in an active game.
    self = { ...self, history: self.history.map(m => ({ ...m, win: null })), profile: { ...self.profile, rankState: "unavailable", ranks: [], updatedAt: "2099-01-01T00:00:00Z" } };
    snapshot = { ...snapshot, phase: "InProgress", stage: "in-game", participants: [self] };
    await emit();
    assert((await page.locator(".home-ranks").innerText()).includes("段位待确认"));
    assert((await page.locator(".home-analysis-main .overview-performance").innerText()).includes("胜负样本待获取"));
    before = calls.length; await emit();
    assert.equal(calls.length, before);
    await screenshot("partial");
    snapshot = { ...snapshot, connectionId: 3, selfKey: "in-game-self", generation: snapshot.generation + 1, participants: [player("in-game-self", 3)] };
    before = calls.length;
    await emit();
    await page.locator(".home-account h2").filter({ hasText: "查询的其他玩家#DEMO" }).waitFor();
    assert.equal(calls.length, before, "in-game startup launched another home batch");
    // Explicit search can wait behind the bounded home request, and can be cancelled.
    for (const cancel of [true, false]) {
      deferHome = true;
      const entered = new Promise(resolve => { homeEntered = resolve; });
      const connection = snapshot.connectionId + 1;
      self = player(`queued-${connection}`, connection);
      snapshot = { ...snapshot, connectionId: connection, selfKey: self.key, stage: 'idle', phase: 'None', participants: [], generation: snapshot.generation + 1 };
      await emit(); await entered;
      const lookups = calls.filter(c => c === 'Lookup').length;
      await page.getByRole('textbox', { name: '玩家完整 ID' }).fill('等待查询#DEMO');
      await page.getByRole('button', { name: '查询战绩', exact: true }).click();
      await page.getByRole('button', { name: '取消等待', exact: true }).waitFor();
      assert.equal(calls.filter(c => c === 'Lookup').length, lookups, 'queued query must not compete with the occupied lookup lane');
      if (cancel) await page.getByRole('button', { name: '取消等待', exact: true }).click();
      deferHome = false; releaseHome();
      if (cancel) {
        await page.locator('.home-heading button').filter({ hasText: '刷新数据' }).waitFor();
        assert.equal(calls.filter(c => c === 'Lookup').length, lookups, 'cancelled query ran after the home request');
      } else {
        await page.locator('.players-workspace .identity-title').filter({ hasText: '查询的其他玩家#DEMO' }).waitFor();
        assert.equal(calls.filter(c => c === 'Lookup').length, lookups + 1);
      }
    }
    // First launch has no cached account. The same inline guide remains usable
    // without requesting CurrentPlayer or opening a modal.
    snapshot = { ...snapshot, connected: false, selfKey: '', participants: [], generation: snapshot.generation + 1 };
    const homeCallsBeforeOfflineLaunch = calls.filter(call => call === 'CurrentPlayer').length;
    await page.reload();
    await connectionGuide.waitFor();
    assert.equal(await page.locator('.home-avatar-empty').count(), 1, 'offline first launch must show an empty account');
    assert.equal(await page.getByRole('dialog').count(), 0);
    await nav.getByRole('button', {name: '游戏辅助', exact: true}).click();
    await nav.getByRole('button', {name: '首页', exact: true}).click();
    await connectionGuide.waitFor();
    assert.equal(calls.filter(call => call === 'CurrentPlayer').length, homeCallsBeforeOfflineLaunch, 'offline first launch requested player data');
    // Assistant starts before the client; a transient first read recovers on its own.
    self = player('cold-start', snapshot.connectionId + 1);
    snapshot = { ...snapshot, connected: true, connectionId: self.connectionId, selfKey: self.key, stage: 'idle', phase: 'None', participants: [] };
    failHome = true;
    await emit();
    await page.locator('.home-error').waitFor();
    const failedHomeCalls = calls.filter(call => call === 'CurrentPlayer').length;
    failHome = false;
    await page.locator('.home-account h2').filter({hasText:self.displayName}).waitFor();
    assert.equal(calls.filter(call => call === 'CurrentPlayer').length, failedHomeCalls + 1, 'cold startup did not recover without refresh');
    assert.deepEqual(errors, []); assert.deepEqual(external, []);
    console.log("PASS: home ownership/bounded startup recovery/cache/refresh/failure/account switch, light/dark/system/persistence, settings focus and 1280/1100/880 layouts; no external requests (synthetic bridge).");
  } finally { await browser.close(); }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
