// Synthetic bridge: account ownership, bounded home preparation and theme choice.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const { mockWindowRuntime } = require("./window-runtime.cjs");

async function main() {
  const browser = await chromium.launch({ headless: true, executablePath: process.env.BROWSER_EXECUTABLE });
  try {
    const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    const errors = [], external = [], calls = [];
    page.on("pageerror", e => errors.push(e.message));
    const desktop = mockWindowRuntime(page);
    const binding = await fs.readFile(path.join(__dirname, "../bindings/leaguepingo/appservice.ts"), "utf8");
    const methods = Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m => [m[2], m[1]]));
    const avatar = "data:image/svg+xml;base64," + Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect width="64" height="64" fill="#6652a8"/><circle cx="32" cy="32" r="18" fill="#efc280"/></svg>').toString("base64");
    function player(key = "synthetic-self", connectionId = 1) {
      return {
        key, puuid: key, connectionId, displayName: key === "synthetic-self" ? "我的合成账号#DEMO" : "查询的其他玩家#DEMO", identityState: "known", historyState: "ready", privacyState: "public", team: "ally", championId: 107,
        history: Array.from({ length: 20 }, (_, i) => ({ gameId: 50000 + i, playedAt: "2026-10-20T03:00:00Z", queueId: i % 3 ? 420 : 440, championId: [107, 23, 25, 99, 17][i % 5], win: i % 4 !== 0, kills: 6 + i % 3, deaths: 3, assists: 8, duration: 1810, items: [3006, 3078], spell1: 4, spell2: 14 })),
        historyMeta: { fetchedAt: "2026-10-20T03:00:00Z", returned: 20, parsed: 20 }, stats: { games: 20 },
        profile: { updatedAt: "2026-10-20T03:00:00Z", level: 128, profileIconData: avatar, privacyState: "public", rankState: "ready", ranks: [{ queue: "RANKED_SOLO_5x5", tier: "EMERALD", division: "II", lp: 48, wins: 65, losses: 47 }, { queue: "RANKED_FLEX_SR", tier: "GOLD", division: "I", lp: 23, wins: 25, losses: 21 }], fields: { rank: { state: "ready", fetchedAt: "2026-10-20T03:00:00Z" } } },
      };
    }
    let self = player(), failHome = false;
    let snapshot = { connectionId: 1, selfKey: self.key, generation: 1, connected: true, phase: "None", stage: "idle", frozen: false, updatedAt: "2026-10-20T03:00:00Z", participants: [] };
    await page.route("**/*", route => { external.push(route.request().url()); return route.abort(); });
    await page.route("http://127.0.0.1:43202/**", async route => {
      const url = new URL(route.request().url());
      if (url.pathname === "/wails/custom.js") return route.fulfill({ contentType: "text/javascript", body: "" });
      if (url.pathname === "/wails/runtime") {
        const request = route.request().postDataJSON();
        if (await desktop.handle(route, request)) return;
        const method = methods[request.args?.methodID];
        calls.push(method);
        let result;
        if (method === "Snapshot") result = snapshot;
        else if (method === "OverlayStatus") result = { visible: false, pinned: true, revision: 0 };
        else if (method === "CurrentPlayer") {
          if (failHome) return route.fulfill({ status: 500, json: { message: "当前账号资料暂不可用" } });
          result = self;
        } else if (method === "Lookup") result = player("synthetic-other");
        else if (method === "RefreshData") { self = { ...self, profile: { ...self.profile, updatedAt: "2026-10-20T03:02:00Z", level: 129 } }; result = self; }
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
      if (process.env.UI_SCREENSHOT_PREFIX) await page.screenshot({ path: `${process.env.UI_SCREENSHOT_PREFIX}-${name}.png` });
    }
    await page.goto("http://127.0.0.1:43202/");
    await page.locator(".home-account h2").filter({ hasText: self.displayName }).waitFor();
    assert.equal(await nav.getByRole("button", { name: "首页", exact: true }).getAttribute("aria-current"), "page");
    assert.equal(await page.locator(".home-match").count(), 5);
    assert.equal(await page.locator(".home-champion").count(), 5);
    assert((await page.locator(".home-ranks").innerText()).includes("翡翠 II"));
    assert((await page.locator(".home-ranks").innerText()).includes("黄金 I"));
    assert.equal(calls.filter(c => c === "CurrentPlayer").length, 1);
    assert(await page.locator(".home-account img").evaluate(img => img.complete && img.naturalWidth > 0));
    await page.getByRole("img", { name: "玩家账号头像", exact: true }).waitFor();
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
    await settings.press("Escape");
    assert(await page.getByRole("button", { name: "设置与诊断", exact: true }).evaluate(el => el === document.activeElement));
    before = calls.length;
    const peer = await page.context().newPage();
    await peer.route("http://127.0.0.1:43202/theme-sync", route => route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Theme storage peer</title>" }));
    await peer.goto("http://127.0.0.1:43202/theme-sync");
    await peer.evaluate(() => localStorage.setItem("leaguepingo.theme", "light"));
    await resolved("light"); assert.equal(await theme.inputValue(), "light");
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
    await page.getByRole("button", { name: "查询战绩", exact: true }).click();
    await page.locator(".players-workspace .identity-title").filter({ hasText: "查询的其他玩家#DEMO" }).waitFor();
    await screenshot("history-dark");
    await nav.getByRole("button", { name: "首页", exact: true }).click();
    assert.equal(await page.locator(".home-account h2").innerText(), "我的合成账号#DEMO");
    before = calls.length;
    snapshot.phase = "Lobby"; snapshot.stage = "party"; snapshot.generation++;
    snapshot.participants = [player("synthetic-other"), self];
    await emit();
    await nav.getByRole("button", { name: "当前对局", exact: true }).click();
    await screenshot("roster-dark");
    await nav.getByRole("button", { name: "首页", exact: true }).click();
    await emit();
    assert.equal(calls.length, before, "cached home navigation or snapshot caused requests");
    for (const size of [{ width: 1100, height: 760 }, { width: 880, height: 620 }]) {
      await page.setViewportSize(size);
      assert(await page.locator(".app-toolbar").evaluate(el => el.scrollWidth <= el.clientWidth + 1), "toolbar overflow");
      assert(await page.locator(".home-workspace").evaluate(el => el.scrollWidth <= el.clientWidth + 1), "home overflow");
      await screenshot(`dark-${size.width}`);
    }
    snapshot.connected = false; snapshot.generation++;
    await emit();
    assert((await page.locator(".home-account").innerText()).includes("旧资料"));
    before = calls.length;
    await theme.selectOption("light"); await resolved("light");
    assert.equal(calls.length, before);
    // A new account cannot inherit the previous user's data; a failed load does
    // not restart on every snapshot, theme change or navigation.
    snapshot = { ...snapshot, connected: true, connectionId: 2, selfKey: "synthetic-new", phase: "None", stage: "idle", generation: snapshot.generation + 1, participants: [] };
    failHome = true;
    await emit();
    await page.locator(".home-error").waitFor();
    assert(!(await page.locator(".home-account").innerText()).includes("我的合成账号"));
    before = calls.filter(c => c === "CurrentPlayer").length;
    await emit(); await nav.getByRole("button", { name: "当前对局", exact: true }).click(); await nav.getByRole("button", { name: "首页", exact: true }).click();
    assert.equal(calls.filter(c => c === "CurrentPlayer").length, before);
    failHome = false; self = player("synthetic-new", 2);
    await page.locator(".home-heading button").click();
    await page.locator(".home-account h2").filter({ hasText: self.displayName }).waitFor();
    // Missing rank and match outcomes remain unknown, rather than becoming zero
    // wins or an unranked claim. No automatic request is made in an active game.
    self = { ...self, history: self.history.map(m => ({ ...m, win: null })), profile: { ...self.profile, rankState: "unavailable", ranks: [], updatedAt: "2099-01-01T00:00:00Z" } };
    snapshot = { ...snapshot, phase: "InProgress", stage: "in-game", participants: [self] };
    await emit();
    assert((await page.locator(".home-ranks").innerText()).includes("来源未提供段位"));
    assert((await page.locator(".home-form-summary").innerText()).includes("暂无胜负样本"));
    before = calls.length; await emit();
    assert.equal(calls.length, before);
    await screenshot("partial");
    snapshot = { ...snapshot, connectionId: 3, selfKey: "in-game-self", generation: snapshot.generation + 1, participants: [player("in-game-self", 3)] };
    before = calls.length;
    await emit();
    await page.locator(".home-account h2").filter({ hasText: "查询的其他玩家#DEMO" }).waitFor();
    assert.equal(calls.length, before, "in-game startup launched another home batch");
    assert.deepEqual(errors, []); assert.deepEqual(external, []);
    console.log("PASS: home ownership/one-shot preparation/cache/refresh/failure/account switch, light/dark/system/persistence, settings focus and 1280/1100/880 layouts; no external requests (synthetic bridge).");
  } finally { await browser.close(); }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
