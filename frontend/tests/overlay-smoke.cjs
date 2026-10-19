// No real accounts or downloads. Uses a locally installed Edge and Playwright.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const { mockWindowRuntime } = require("./window-runtime.cjs");

async function main() {
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.BROWSER_EXECUTABLE,
  });
  try {
    const page = await browser.newPage({
      viewport: { width: 1100, height: 760 },
    });
    page.setDefaultTimeout(15000);
    const errors = [],
      calls = [],
      external = [];
    const desktopWindow = mockWindowRuntime(page);
    page.on("pageerror", (e) => errors.push(e.message));
    await page.route("**/*", (route) => {
      external.push(route.request().url());
      return route.abort();
    });
    const binding = await fs.readFile(
      path.join(__dirname, "../bindings/leaguepingo/appservice.ts"),
      "utf8",
    );
    const methods = Object.fromEntries(
      [
        ...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g),
      ].map((m) => [m[2], m[1]]),
    );
    const history = Array.from({ length: 20 }, (_, i) => ({
      gameId: i + 1,
      queueId: i % 2 ? 450 : 420,
      championId: 23 + (i % 3),
      win: i === 0 ? null : i % 3 !== 0,
      playedAt:
        i === 0 ? null : new Date(Date.now() - i * 3600000).toISOString(),
      kills: i === 0 ? null : i,
      deaths: 2,
      assists: 8,
      duration: 1800,
      cs: 152,
      gold: 12345,
      damage: null,
      items: [3006, 0, null, 3078, 0, 0, 3364],
      spell1: 4,
      spell2: 14,
      mapId: i === 0 ? 11 : null,
      position: i === 0 ? "UTILITY" : "",
      earlyEnd: i === 0 ? true : null,
      runeIds: i === 0 ? [8005, 999999] : [],
      runePrimary: i === 0 ? 8000 : null,
      runeSecondary: i === 0 ? 8100 : null,
    }));
    const stats = {
      games: 20,
      wins: 12,
      losses: 7,
      outcomeGames: 19,
      kdaGames: 19,
      winRate: 63.16,
      kda: 4.2,
    };
    function player(name, key = name) {
      return {
        key,
        puuid: key,
        displayName: name,
        identityState: "known",
        privacyState: "unknown",
        historyState: "ready",
        history,
        stats,
        historyMeta: {
          returned: 21,
          parsed: 20,
          excluded: 1,
          fetchedAt: new Date().toISOString(),
          scope: "最近获取的至多 20 场，所有模式",
        },
      };
    }
    let snapshot = {
      connected: true,
      stage: "in-game",
      phase: "InProgress",
      frozen: true,
      knownCount: 10,
      readyCount: 5,
      participants: Array.from({ length: 10 }, (_, i) => ({
        ...player(`测试玩家${i}#TEST`, `test-${i}`),
        team: i < 5 ? "ally" : "enemy",
        championId: 23,
        historyState: i < 5 ? "ready" : "pending",
        history: i < 5 ? history : [],
      })),
    };
    let state = {
      revision: 0,
      visible: true,
      pinned: true,
      shortcutAvailable: true,
      warning: "",
    };
    let rejectHold = true;
    let releaseRefresh;
    let releasePlayerAction;
    let delayEnsure = false,
      releaseEnsure;
    let info = {
      version: "0.2.0-dev",
      commit: "synthetic",
      buildTime: "test",
      shortcut: "Ctrl+Shift+P",
      settingsWarning: "",
      goHeapBytes: 10485760,
      goroutines: 20,
      requests: {
        history: {
          calls: 10,
          failed: 1,
          totalMs: 500,
          maxMs: 100,
          lastStatus: 200,
        },
      },
      performanceScope: "仅 Go 堆，不包含 WebView2",
    };
    await page.route("http://127.0.0.1:43199/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname === "/wails/custom.js")
        return route.fulfill({ contentType: "text/javascript", body: "" });
      if (url.pathname === "/wails/runtime") {
        if (await desktopWindow.handle(route, route.request().postDataJSON()))
          return;
        const request = route.request().postDataJSON(),
          method = methods[request.args?.methodID],
          args = request.args?.args ?? [];
        calls.push(method);
        let result = null;
        if (method === "Snapshot") result = snapshot;
        else if (method === "Info") result = info;
        else if (method === "ConfigureHold") {
          if (rejectHold)
            return route.fulfill({
              status: 400,
              json: {
                kind: "RuntimeError",
                message: "按住设置保存失败，原设置保留",
              },
            });
          result = info = { ...info, holdEnabled: args[0], holdKey: args[1] };
        } else if (method === "ConfigureShortcut") {
          if (args[0] === "Ctrl+Shift+X")
            return route.fulfill({
              status: 400,
              json: {
                kind: "RuntimeError",
                message: "快捷键注册失败，可能被占用；原设置保留",
              },
            });
          result = info = { ...info, shortcut: args[0] };
        } else if (method === "ExportDiagnostics")
          result = "C:/synthetic/diagnostics/summary.json";
        else if (method === "OverlayStatus") result = state;
        else if (method === "SetOverlayPinned")
          result = state = {
            ...state,
            revision: state.revision + 1,
            pinned: args[0],
          };
        else if (method === "HideOverlay")
          result = state = {
            ...state,
            revision: state.revision + 1,
            visible: false,
          };
        else if (method === "RefreshRoster") result = snapshot;
        else if (method === "EnsureProfile") {
          if (delayEnsure && args[0] === "avatar-ready")
            await new Promise((resolve) => {
              releaseEnsure = resolve;
            });
          const original =
            snapshot.participants.find((p) => p.key === args[0]) ??
            player(args[0]);
          result = {
            ...original,
            profile: {
              updatedAt: new Date(Date.now() + 1000).toISOString(),
              profileIconData:
                "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aP1sAAAAASUVORK5CYII=",
              level: 99,
              privacyState: "unknown",
              rankState: "unavailable",
              ranks: [],
            },
          };
        } else if (method === "LoadProfile" && args[0] === "Beta#TEST") {
          await new Promise((resolve) => {
            releasePlayerAction = resolve;
          });
          return route.fulfill({
            status: 400,
            json: { kind: "RuntimeError", message: "Beta profile failed" },
          });
        } else if (method === "LoadProfile" && args[0] === "test-6")
          result = {
            ...snapshot.participants.find((p) => p.key === args[0]),
            privacyState: "private",
            profile: {
              updatedAt: new Date(Date.now() + 2000).toISOString(),
              privacyState: "private",
              rankState: "unavailable",
              ranks: [],
            },
          };
        else if (method === "LoadProfile")
          result = {
            ...player("Alpha#TEST", args[0]),
            privacyState: "private",
            profile: {
              profileIconData:
                "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aP1sAAAAASUVORK5CYII=",
              level: 100,
              rankState: "unavailable",
              privacyState: "private",
              ranks: [],
            },
          };
        else if (method === "HistoryPage") {
          if (args[2] !== 21)
            throw new Error("pagination must advance by raw returned count");
          result = {
            matches: [
              { ...history[19], kills: null },
              { ...history[1], gameId: 101 },
            ],
            meta: {
              returned: 2,
              parsed: 2,
              excluded: 0,
              fetchedAt: new Date().toISOString(),
            },
            offset: 21,
            nextOffset: 23,
            hasMore: false,
          };
        } else if (method === "MatchDetail")
          result = {
            gameId: args[2],
            partial: false,
            fetchedAt: new Date().toISOString(),
            players: Array.from({ length: 10 }, (_, i) => ({
              participantId: i + 1,
              teamId: i < 5 ? 100 : 200,
              displayName: `详情玩家${i}#TEST`,
              target: i === 0,
              summary: history[1],
              damageShare: 20,
              goldShare: 20,
              participation: 40,
            })),
          };
        else if (
          method === "Lookup" ||
          method === "RefreshLookup" ||
          method === "RefreshHistory"
        ) {
          if (method === "RefreshHistory" && args[0] === "Alpha#TEST") {
            await new Promise((resolve) => {
              releasePlayerAction = resolve;
            });
            return route.fulfill({
              status: 400,
              json: { kind: "RuntimeError", message: "Alpha history failed" },
            });
          }
          if (method === "RefreshHistory" && args[0] === "test-5") {
            await new Promise((resolve) => {
              releaseRefresh = resolve;
            });
            return route.fulfill({
              status: 400,
              json: { kind: "RuntimeError", message: "A query failed" },
            });
          }
          if (args[0] === "Missing#TEST")
            return route.fulfill({
              status: 400,
              json: {
                kind: "RuntimeError",
                message: "无法确认此 ID 对应的账号",
              },
            });
          result = player(args[0]);
        } else if (method === "LookupPlayer" || method === "RefreshPlayer") {
          result = player("测试玩家5#TEST", args[0]);
          snapshot = { ...snapshot, participants: [] };
          await page.evaluate(
            (s) =>
              window._wails.dispatchWailsEvent({
                name: "leaguepingo:snapshot",
                data: s,
              }),
            snapshot,
          );
        }
        return route.fulfill({ json: result });
      }
      const relative =
        url.pathname === "/" ? "index.html" : url.pathname.slice(1);
      if (relative.includes("..")) return route.abort();
      const body = await fs.readFile(path.join(__dirname, "../dist", relative));
      return route.fulfill({
        body,
        contentType: relative.endsWith(".js")
          ? "text/javascript"
          : relative.endsWith(".css")
            ? "text/css"
            : relative.endsWith(".png")
              ? "image/png"
              : "text/html",
      });
    });
    await page.goto("http://127.0.0.1:43199/");
    await page.locator(".desktop-titlebar").waitFor();
    await page.waitForFunction(() =>
      document.querySelector(".toolbar-status")?.textContent.includes("对局中"),
    );
    const chromeServiceCalls = calls.length;
    await page.getByRole("button", { name: "最小化窗口", exact: true }).click();
    await page.getByRole("button", { name: "最大化窗口", exact: true }).click();
    await page.getByRole("button", { name: "还原窗口", exact: true }).waitFor();
    await page
      .getByRole("button", { name: "还原窗口", exact: true })
      .press("Enter");
    await page
      .getByRole("button", { name: "最大化窗口", exact: true })
      .waitFor();
    assert(
      await page
        .getByRole("button", { name: "最大化窗口", exact: true })
        .evaluate((el) => el === document.activeElement),
      "keyboard window control must preserve focus",
    );
    await page.locator(".window-brand").dblclick();
    await page.getByRole("button", { name: "还原窗口", exact: true }).waitFor();
    await page.getByRole("button", { name: "还原窗口", exact: true }).click();
    await page
      .getByRole("button", { name: "最大化窗口", exact: true })
      .waitFor();
    await page
      .getByRole("button", { name: "关闭窗口并收起到托盘", exact: true })
      .click();
    assert(desktopWindow.calls.includes("Minimise"));
    assert(
      desktopWindow.calls.includes("Close"),
      "custom close must use the native closing hook",
    );
    assert.equal(
      desktopWindow.calls.filter((m) => m === "ToggleMaximise").length,
      4,
      "caption double click and keyboard controls must toggle once",
    );
    await desktopWindow.emit("WindowMaximise", "overlay");
    assert.equal(
      await page
        .getByRole("button", { name: "最大化窗口", exact: true })
        .count(),
      1,
    );
    desktopWindow.maximised = true;
    await desktopWindow.emit("WindowMaximise");
    await page.getByRole("button", { name: "还原窗口", exact: true }).waitFor();
    desktopWindow.maximised = false;
    await desktopWindow.emit("WindowUnMaximise");
    await page
      .getByRole("button", { name: "最大化窗口", exact: true })
      .waitFor();
    await desktopWindow.emit("WindowLostFocus");
    assert(await page.locator(".desktop-titlebar.window-inactive").count());
    await desktopWindow.emit("WindowFocus");
    await page.waitForFunction(
      () =>
        !document
          .querySelector(".desktop-titlebar")
          .classList.contains("window-inactive"),
    );
    desktopWindow.failNext = "ToggleMaximise";
    await page.getByRole("button", { name: "最大化窗口", exact: true }).click();
    await page
      .getByRole("status")
      .filter({ hasText: "窗口操作未完成" })
      .waitFor();
    await page.getByRole("button", { name: "最大化窗口", exact: true }).click();
    await page.getByRole("button", { name: "还原窗口", exact: true }).waitFor();
    await page.getByRole("button", { name: "还原窗口", exact: true }).click();
    assert.equal(
      calls.length,
      chromeServiceCalls,
      "window controls must not trigger game or app service requests",
    );
    assert.equal(await page.locator(".window-control-error").count(), 0);
    assert.deepEqual(
      await page.evaluate(() => [
        getComputedStyle(document.querySelector(".desktop-titlebar"))
          .getPropertyValue("--wails-draggable")
          .trim(),
        getComputedStyle(document.querySelector(".window-controls button"))
          .getPropertyValue("--wails-draggable")
          .trim(),
      ]),
      ["drag", "no-drag"],
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-window.png",
      });
    async function search(id) {
      await page.getByLabel("玩家完整 ID", { exact: true }).fill(id);
      await page.getByRole("button", { name: "查询战绩", exact: true }).click();
    }
    for (const id of [
      "\u2066\u2069#TEST",
      "Alpha#\u2066\u2069",
      "Alpha#TEST#extra",
    ]) {
      const lookups = calls.filter((c) => c === "Lookup").length;
      await search(id);
      await page
        .getByText("请输入完整的 玩家名#标签", { exact: true })
        .waitFor();
      assert.equal(
        calls.filter((c) => c === "Lookup").length,
        lookups,
        "invalid copied ID reached the bridge",
      );
    }
    const invalidNotice = page.getByText("请输入完整的 玩家名#标签", {
      exact: true,
    });
    await page.evaluate(() => {
      const observer = new MutationObserver(() => {
        const el = document.querySelector(".notice-leave-active");
        if (!el) return;
        observer.disconnect();
        const initialHeight = el.getBoundingClientRect().height;
        setTimeout(() => {
          window.__noticeExit = {
            duration: getComputedStyle(el).transitionDuration,
            opacity: Number(getComputedStyle(el).opacity),
            height: el.getBoundingClientRect().height,
            initialHeight,
          };
        }, 90);
      });
      observer.observe(document.body, {
        subtree: true,
        attributes: true,
        attributeFilter: ["class"],
      });
    });
    await page.waitForTimeout(1100);
    await search("Alpha#TEST#extra");
    await page.waitForTimeout(1100);
    assert(
      await invalidNotice.isVisible(),
      "repeated notice did not restart its two-second timer",
    );
    await invalidNotice.waitFor({ state: "hidden", timeout: 1800 });
    const noticeExit = await page.evaluate(() => window.__noticeExit);
    assert(
      noticeExit?.duration.includes("0.2s") &&
        noticeExit.opacity > 0 &&
        noticeExit.opacity < 1 &&
        noticeExit.height < noticeExit.initialHeight,
      "notice did not fade/collapse progressively",
    );
    await page.emulateMedia({ reducedMotion: "reduce" });
    await search("Alpha#");
    const reducedDuration = await page
      .locator(".transient-notice")
      .evaluate((el) => {
        el.classList.add("notice-enter-active");
        const duration = getComputedStyle(el).transitionDuration;
        el.classList.remove("notice-enter-active");
        return duration;
      });
    assert.equal(reducedDuration, "0s", "reduced motion ignored");
    await page.emulateMedia({ reducedMotion: "no-preference" });
    await search("Alpha#");
    await page.waitForFunction(
      () => !!document.querySelector(".notice-leave-active"),
    );
    await search("Alpha#");
    await page.waitForTimeout(300);
    assert(
      await invalidNotice.isVisible(),
      "new error was removed by the preceding exit animation",
    );
    await invalidNotice.waitFor({ state: "hidden", timeout: 3000 });
    await search("\u2066Alpha\u2069#\u2066TEST\u2069");
    await page
      .getByRole("heading", { name: "Alpha#TEST", exact: true })
      .waitFor();
    assert.equal(
      await page.getByLabel("玩家完整 ID", { exact: true }).inputValue(),
      "Alpha#TEST",
    );
    assert.equal(await page.locator(".match-card").count(), 20);
    const beforeDodge = snapshot;
    snapshot = {
      ...snapshot,
      generation: (snapshot.generation ?? 0) + 1,
      stage: "party",
      phase: "Lobby",
      updatedAt: new Date().toISOString(),
      participants: [
        {
          ...player("Alpha#TEST", "Alpha#TEST"),
          displayName: "玩家资料加载中",
          team: "ally",
        },
      ],
    };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:snapshot",
          data: s,
        }),
      snapshot,
    );
    await page.waitForFunction(() =>
      document
        .querySelector(".toolbar-status")
        ?.textContent.includes("组队大厅"),
    );
    await page.getByRole("tab", { name: "Alpha#TEST", exact: true }).waitFor();
    assert.equal(
      await page
        .getByRole("heading", { name: "Alpha#TEST", exact: true })
        .count(),
      1,
      "lobby placeholder overwrote a verified queried player's name",
    );
    snapshot = {
      ...beforeDodge,
      generation: snapshot.generation + 1,
      updatedAt: new Date().toISOString(),
    };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:snapshot",
          data: s,
        }),
      snapshot,
    );
    await search("Alpha#");
    await invalidNotice.waitFor();
    assert(
      !(await page.locator(".workspace-message").allTextContents())
        .join("")
        .includes("已有资料保留"),
    );
    await invalidNotice.waitFor({ state: "hidden", timeout: 3000 });
    assert.equal(
      await page.locator(".match-card").count(),
      20,
      "dismissal cleared the existing history",
    );
    const queueOptions = await page
      .getByLabel("战绩模式筛选")
      .locator("option")
      .allTextContents();
    for (const label of ["单双排", "灵活排位", "海克斯大乱斗", "极地大乱斗"])
      assert(queueOptions.includes(label), `missing queue: ${label}`);
    assert.equal(
      await page
        .locator(".match-main")
        .first()
        .locator(".match-items .game-icon")
        .count(),
      7,
      "equipment must be visible without expanding",
    );
    assert.equal(
      await page
        .locator(".match-main")
        .first()
        .locator(".match-spells .game-icon")
        .count(),
      2,
      "spells must be visible without expanding",
    );
    assert(
      !calls.includes("MatchDetail"),
      "rendering rich rows must not fetch full matches",
    );
    await page.setViewportSize({ width: 1280, height: 800 });
    const standardRows = await page.locator(".match-card").evaluateAll(
      (rows) =>
        rows.filter((row) => {
          const b = row.getBoundingClientRect(),
            viewport = row.closest(".history-scroll").getBoundingClientRect();
          return b.top >= viewport.top && b.bottom <= viewport.bottom;
        }).length,
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-players.png",
      });
    assert(
      standardRows >= 5,
      `1280x800 must show five complete matches, got ${standardRows}`,
    );
    await page.setViewportSize({ width: 1100, height: 760 });
    await page
      .getByRole("button", { name: "更新玩家资料", exact: true })
      .click();
    await page
      .getByText("生涯不公开（客户端明确返回）", { exact: false })
      .waitFor();
    await page.waitForFunction(() => {
      const img = document.querySelector(".profile-avatar img");
      return img?.complete && img.naturalWidth > 0;
    });
    const visibleRows = await page.locator(".match-card").evaluateAll(
      (rows) =>
        rows.filter((row) => {
          const b = row.getBoundingClientRect();
          return b.top >= 0 && b.bottom <= innerHeight;
        }).length,
    );
    assert(
      visibleRows >= 4,
      `expected at least four complete rows in first viewport, got ${visibleRows}`,
    );
    await page
      .locator(".quality-warning summary")
      .filter({ hasText: "未纳入统计" })
      .waitFor();
    await page.getByLabel("战绩模式筛选").selectOption("420");
    await page.waitForFunction(
      () => !document.querySelector('[aria-label="战绩模式筛选"]').disabled,
    );
    assert.equal(await page.locator(".match-card").count(), 10);
    await page.locator(".history-notice summary").click();
    assert(
      (await page.locator(".history-notice").textContent()).includes(
        "胜率按 9 场",
      ),
      "notice must use the filtered denominator",
    );
    await page.locator(".match-main").first().click();
    assert.equal(await page.locator(".match-expanded").count(), 1);
    await page
      .getByLabel("对局上下文")
      .getByText("召唤师峡谷", { exact: true })
      .waitFor();
    await page
      .getByLabel("对局上下文")
      .getByText("辅助", { exact: true })
      .waitFor();
    await page
      .getByLabel("对局上下文")
      .getByText("提前结束", { exact: true })
      .waitFor();
    await page
      .getByLabel("本玩家符文")
      .getByText("强攻", { exact: true })
      .waitFor();
    await page
      .getByLabel("本玩家符文")
      .getByText("符文 #999999", { exact: true })
      .waitFor();
    await page
      .getByLabel("本玩家符文")
      .getByText("精密 / 主宰", { exact: true })
      .waitFor();
    await page
      .locator(".match-expanded")
      .getByText("金币 12345", { exact: true })
      .waitFor();
    assert(
      !calls.includes("MatchDetail"),
      "expanding a match must not fetch full detail automatically",
    );
    await page
      .getByRole("button", { name: "读取双方详情", exact: true })
      .click();
    await page.waitForFunction(
      () =>
        document.querySelectorAll(".full-match-detail tbody tr").length === 10,
    );
    assert.equal(await page.locator(".full-match-detail tr.target").count(), 1);
    assert.equal(
      await page.locator(".full-match-detail tbody .detail-loadout").count(),
      20,
      "both teams must expose received equipment and spells",
    );
    assert.equal(
      await page
        .locator(".match-main")
        .first()
        .locator(".match-items .game-icon")
        .count(),
      7,
    );
    assert(
      await page
        .locator(".match-hero .game-icon")
        .first()
        .evaluate((el) =>
          getComputedStyle(el).backgroundImage.includes("/assets/"),
        ),
    );
    await search("Beta#TEST");
    await page
      .getByRole("heading", { name: "Beta#TEST", exact: true })
      .waitFor();
    await page.getByRole("tab", { name: "Alpha#TEST", exact: true }).click();
    const identityCalls = calls.filter(
      (c) => c === "Lookup" || c === "RefreshLookup",
    ).length;
    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    assert.equal(
      await page
        .locator(".global-search button")
        .textContent()
        .then((t) => t.trim()),
      "查询战绩",
      "refresh must not animate top-level search",
    );
    await page.getByRole("tab", { name: "Beta#TEST", exact: true }).click();
    for (let i = 0; !releasePlayerAction && i < 500; i++)
      await new Promise((resolve) => setTimeout(resolve, 10));
    assert(releasePlayerAction, "history refresh did not reach bridge");
    assert.equal(
      calls.filter((c) => c === "Lookup" || c === "RefreshLookup").length,
      identityCalls,
      "refresh repeated a name lookup",
    );
    releasePlayerAction();
    releasePlayerAction = undefined;
    await page.waitForFunction(
      () => !document.querySelector(".history-actions button")?.disabled,
    );
    assert(
      !(await page.locator(".players-workspace").textContent()).includes(
        "Alpha history failed",
      ),
      "A history failure leaked to B",
    );
    await page.getByRole("tab", { name: "Alpha#TEST", exact: true }).click();
    await page
      .locator(".player-error")
      .filter({ hasText: "Alpha history failed" })
      .waitFor();
    await page.getByRole("tab", { name: "Beta#TEST", exact: true }).click();
    await page
      .getByRole("button", { name: "更新玩家资料", exact: true })
      .click();
    await page.getByRole("tab", { name: "Alpha#TEST", exact: true }).click();
    for (let i = 0; !releasePlayerAction && i < 500; i++)
      await new Promise((resolve) => setTimeout(resolve, 10));
    assert(releasePlayerAction, "profile refresh did not reach bridge");
    releasePlayerAction();
    releasePlayerAction = undefined;
    await page.waitForFunction(
      () => !document.querySelector(".history-actions button")?.disabled,
    );
    assert(
      !(await page.locator(".players-workspace").textContent()).includes(
        "Beta profile failed",
      ),
      "B profile failure leaked to A",
    );
    await page.getByRole("tab", { name: "Beta#TEST", exact: true }).click();
    await page
      .locator(".player-error")
      .filter({ hasText: "Beta profile failed" })
      .waitFor();
    await page
      .locator(".player-error")
      .waitFor({ state: "hidden", timeout: 3000 });
    await page.getByRole("tab", { name: "Alpha#TEST", exact: true }).click();
    assert.equal(await page.getByLabel("战绩模式筛选").inputValue(), "420");
    assert.equal(await page.locator(".match-expanded").count(), 1);
    await page.getByLabel("战绩模式筛选").selectOption("all");
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await page.waitForFunction(
      () => document.querySelectorAll(".match-card").length === 1,
    );
    await page.getByRole("button", { name: "上一页", exact: true }).click();
    assert.equal(await page.locator(".match-card").count(), 20);
    assert(
      (await page
        .locator(".match-card")
        .filter({ hasText: "19 / 2 / 8" })
        .count()) === 1,
      "an overlapping page must not erase already verified fields",
    );
    assert.equal(
      await page
        .getByRole("button", { name: "加载更多战绩", exact: true })
        .count(),
      0,
    );
    await page.getByLabel("战绩模式筛选").selectOption("420");
    await search("Missing#TEST");
    await page.getByRole("alert").filter({ hasText: "无法确认" }).waitFor();
    assert.equal(
      await page
        .getByRole("heading", { name: "Alpha#TEST", exact: true })
        .count(),
      1,
    );
    if (process.env.WORKSPACE_SCREENSHOT)
      await page.screenshot({ path: process.env.WORKSPACE_SCREENSHOT });
    await page.setViewportSize({ width: 880, height: 620 });
    assert(
      await page.locator(".window-controls").evaluate((el) => {
        const rect = el.getBoundingClientRect();
        const titlebar = el
          .closest(".desktop-titlebar")
          .getBoundingClientRect();
        return (
          rect.top === 0 &&
          rect.right <= innerWidth &&
          rect.bottom <= titlebar.bottom
        );
      }),
      "window controls must remain at the top edge at minimum size",
    );
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      ),
      false,
    );
    await page.setViewportSize({ width: 1100, height: 760 });
    for (let i = 0; i < 6; i++) {
      await search(`Extra${i}#TEST`);
      await page
        .getByRole("heading", { name: `Extra${i}#TEST`, exact: true })
        .waitFor();
    }
    assert.equal(await page.getByRole("tab").count(), 8);
    const navigationCalls = calls.length;
    async function assertFocusedTab(index) {
      await page.waitForFunction((i) => {
        const tabs = document.querySelectorAll('[role="tab"]');
        const tab = tabs[i];
        const panel = document.getElementById("player-panel");
        return (
          tab?.getAttribute("aria-selected") === "true" &&
          document.activeElement === tab &&
          tab.tabIndex === 0 &&
          panel?.getAttribute("aria-labelledby") === tab.id &&
          [...tabs].filter((t) => t.tabIndex === 0).length === 1
        );
      }, index);
    }
    await page.getByRole("tab", { selected: true }).press("Home");
    await assertFocusedTab(0);
    await page.getByRole("tab", { selected: true }).press("ArrowLeft");
    await assertFocusedTab(7);
    await page.getByRole("tab", { selected: true }).press("ArrowRight");
    await assertFocusedTab(0);
    await page.getByRole("tab", { selected: true }).press("End");
    await assertFocusedTab(7);
    assert.equal(
      calls.length,
      navigationCalls,
      "keyboard navigation must only switch cached views",
    );
    await page.getByRole("tab", { selected: true }).press("Delete");
    assert.equal(await page.getByRole("tab").count(), 7);
    await assertFocusedTab(6);
    await search("Extra5#TEST");
    await page
      .getByRole("heading", { name: "Extra5#TEST", exact: true })
      .waitFor();
    await page.getByRole("tab", { name: "Alpha#TEST", exact: true }).click();
    snapshot = {
      ...snapshot,
      generation: 10,
      updatedAt: new Date().toISOString(),
    };
    await page.evaluate((s) => {
      window._wails.dispatchWailsEvent({
        name: "leaguepingo:snapshot",
        data: s,
      });
      window._wails.dispatchWailsEvent({
        name: "leaguepingo:snapshot",
        data: { ...s, generation: 9, phase: "Lobby" },
      });
    }, snapshot);
    assert(
      (await page.locator(".toolbar-status").textContent()).includes("对局中"),
      "old snapshots must not overwrite new game state",
    );
    await page
      .getByRole("navigation", { name: "工作区" })
      .getByRole("button", { name: /当前对局/ })
      .click();
    await page
      .getByRole("button", { name: "刷新对局名单", exact: true })
      .click();
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-match.png",
      });
    await page.locator(".player-row").nth(5).click();
    await page
      .getByRole("button", { name: "手动查询此玩家", exact: true })
      .click();
    await page
      .locator(".player-drawer")
      .getByRole("heading", { name: "测试玩家5#TEST" })
      .waitFor();
    assert.equal(
      await page.locator(".player-drawer .match-card").count(),
      20,
      "successful selected-player result must survive roster replacement",
    );
    snapshot = {
      ...snapshot,
      generation: 11,
      updatedAt: new Date().toISOString(),
      participants: [
        {
          ...player("测试玩家5#TEST", "test-5"),
          team: "enemy",
          history: history.slice(0, 1),
          historyMeta: { fetchedAt: new Date(Date.now() + 1000).toISOString() },
        },
        { ...player("测试玩家6#TEST", "test-6"), team: "enemy" },
      ],
    };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:snapshot",
          data: s,
        }),
      snapshot,
    );
    await page.waitForFunction(
      () =>
        document.querySelectorAll(".player-drawer .match-card").length === 1,
    );
    await page
      .locator(".player-drawer")
      .getByRole("button", { name: "刷新战绩", exact: true })
      .click();
    await page.waitForFunction(
      () =>
        document.querySelector(".player-drawer .history-actions button")
          ?.disabled,
    );
    await page.locator(".player-row").nth(1).click();
    for (let i = 0; !releaseRefresh && i < 500; i++)
      await new Promise((resolve) => setTimeout(resolve, 10));
    assert(releaseRefresh, "refresh request did not reach the bridge");
    releaseRefresh();
    await page.waitForFunction(
      () =>
        !document.querySelector(".player-drawer .history-actions button")
          ?.disabled,
    );
    assert(
      !(await page.locator(".player-drawer").textContent()).includes(
        "A query failed",
      ),
      "A failure must not appear on B",
    );
    await page
      .locator(".player-drawer")
      .getByRole("button", { name: "更新玩家资料", exact: true })
      .click();
    await page
      .locator(".player-drawer")
      .getByText("生涯不公开（客户端明确返回）", { exact: false })
      .waitFor();
    snapshot = {
      ...snapshot,
      updatedAt: new Date(Date.now() + 1000).toISOString(),
      participants: snapshot.participants.map((p) => ({
        ...p,
        privacyState: "public",
        profile: {
          updatedAt: "2026-01-01T00:00:00Z",
          privacyState: "public",
          rankState: "unavailable",
          ranks: [],
        },
      })),
    };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:snapshot",
          data: s,
        }),
      snapshot,
    );
    assert(
      (await page.locator(".player-drawer").textContent()).includes(
        "生涯不公开（客户端明确返回）",
      ),
      "older profile with equal history timestamp overwrote explicit update",
    );
    await page
      .getByRole("navigation", { name: "工作区" })
      .getByRole("button", { name: /玩家战绩/ })
      .click();
    assert.equal(
      await page.locator(".players-workspace .player-profile h1").textContent(),
      (await page.getByRole("tab", { selected: true }).textContent()).trim(),
      "background results must keep a valid active tab after eviction",
    );
    assert(calls.includes("LookupPlayer") && calls.includes("RefreshRoster"));
    await page.getByRole("button", { name: "设置与诊断", exact: true }).click();
    await page.getByRole("dialog", { name: "设置与诊断" }).waitFor();
    assert.deepEqual(
      await page
        .locator(".window-controls button")
        .evaluateAll((buttons) => buttons.map((button) => button.tabIndex)),
      [-1, -1, -1],
    );
    await page.getByRole("button", { name: "最小化窗口", exact: true }).click();
    assert(
      await page
        .getByRole("dialog", { name: "设置与诊断" })
        .evaluate((el) => el === document.activeElement),
      "caption controls must remain available without stealing modal focus",
    );
    assert.equal(await page.locator(".app-main").getAttribute("inert"), "");
    await page.getByRole("dialog", { name: "设置与诊断" }).press("Shift+Tab");
    assert.equal(
      await page
        .getByRole("button", { name: "导出脱敏诊断" })
        .evaluate((el) => el === document.activeElement),
      true,
    );
    await page.keyboard.press("Tab");
    assert.equal(
      await page
        .getByRole("button", { name: "关闭设置" })
        .evaluate((el) => el === document.activeElement),
      true,
    );
    await page.getByText(/0.2.0-dev · synthetic/).waitFor();
    await page
      .getByLabel("快览快捷键", { exact: true })
      .selectOption("Ctrl+Shift+X");
    await page.getByRole("button", { name: "保存快捷键" }).click();
    await page.getByRole("alert").filter({ hasText: "原设置保留" }).waitFor();
    assert.equal(info.shortcut, "Ctrl+Shift+P");
    await page
      .getByLabel("快览快捷键", { exact: true })
      .selectOption("Ctrl+Shift+L");
    await page.getByRole("button", { name: "保存快捷键" }).click();
    await page
      .getByText("快捷键已保存；窗口位置和置顶状态也会记住。")
      .waitFor();
    await page.getByLabel("启用局内按住快览").uncheck();
    await page.getByLabel("局内按住键").selectOption("F9");
    await page.getByRole("button", { name: "保存按住设置" }).click();
    await page
      .getByRole("alert")
      .filter({ hasText: "按住设置保存失败" })
      .waitFor();
    assert.notEqual(info.holdKey, "F9");
    rejectHold = false;
    await page.getByRole("button", { name: "保存按住设置" }).click();
    await page.getByText("局内按住快览设置已保存。").waitFor();
    assert.equal(info.holdKey, "F9");
    assert.equal(info.holdEnabled, false);
    await page.getByRole("button", { name: "导出脱敏诊断" }).click();
    await page.getByText(/已保存本地诊断/).waitFor();
    await page.getByRole("button", { name: "关闭设置" }).click();
    assert.deepEqual(
      await page
        .locator(".window-controls button")
        .evaluateAll((buttons) => buttons.map((button) => button.tabIndex)),
      [0, 0, 0],
    );
    await page.waitForFunction(
      () => document.activeElement?.getAttribute("aria-label") === "设置与诊断",
    );
    await page.getByRole("button", { name: "设置与诊断", exact: true }).click();
    await page.getByRole("dialog", { name: "设置与诊断" }).press("Escape");
    await page.waitForFunction(
      () =>
        !document.querySelector('[role="dialog"]') &&
        document.activeElement?.getAttribute("aria-label") === "设置与诊断",
    );
    const avatarCalls = calls.filter((c) =>
      /Lookup|History|Profile/.test(c),
    ).length;
    const avatarData =
      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aP1sAAAAASUVORK5CYII=";
    snapshot = {
      ...snapshot,
      generation: 100,
      stage: "preparing",
      phase: "ChampSelect",
      knownCount: 3,
      readyCount: 3,
      participants: [
        {
          ...player("未选英雄#TEST", "avatar-ready"),
          team: "ally",
          championState: "unselected",
          profile: { profileIconData: avatarData },
        },
        {
          ...player("未读到英雄#TEST", "avatar-missing"),
          team: "ally",
          championState: "unavailable",
        },
        {
          ...player("已选英雄#TEST", "avatar-champion"),
          team: "enemy",
          championId: 23,
          championState: "selected",
          profile: { profileIconData: avatarData },
        },
      ],
    };
    async function publishSnapshot() {
      snapshot = {
        ...snapshot,
        generation: snapshot.generation + 1,
        updatedAt: new Date().toISOString(),
      };
      await page.evaluate(
        (s) =>
          window._wails.dispatchWailsEvent({
            name: "leaguepingo:snapshot",
            data: s,
          }),
        snapshot,
      );
    }
    await publishSnapshot();
    await page
      .getByRole("navigation", { name: "工作区" })
      .getByRole("button", { name: /当前对局/ })
      .click();
    await page.waitForFunction(
      () => document.querySelectorAll(".player-row").length === 3,
    );
    const rosterRows = page.locator(".player-row");
    await page.waitForFunction(() => {
      const img = document.querySelector(".player-row .participant-avatar img");
      return img?.complete && img.naturalWidth > 0;
    });
    assert.equal(
      await rosterRows.nth(0).locator(".participant-avatar img").count(),
      1,
    );
    assert((await rosterRows.nth(0).textContent()).includes("暂未选择英雄"));
    assert.equal(
      await rosterRows.nth(1).locator(".participant-avatar .app-icon").count(),
      1,
    );
    assert(
      (await rosterRows.nth(1).textContent()).includes("英雄信息暂不可用"),
    );
    assert.equal(
      await rosterRows.nth(2).locator(".participant-avatar .game-icon").count(),
      1,
    );
    for (const hero of [24, 25, 0]) {
      snapshot.participants[0] = {
        ...snapshot.participants[0],
        championId: hero,
        championState: hero ? "selected" : "unselected",
      };
      await publishSnapshot();
      if (hero)
        await rosterRows
          .nth(0)
          .locator(".participant-avatar .game-icon")
          .waitFor();
      else await rosterRows.nth(0).locator(".participant-avatar img").waitFor();
    }
    await rosterRows
      .nth(0)
      .locator(".participant-avatar img")
      .dispatchEvent("error");
    await rosterRows.nth(0).locator(".participant-avatar .app-icon").waitFor();
    snapshot.participants[1] = {
      ...snapshot.participants[1],
      championId: 24,
      championState: "intent",
    };
    await publishSnapshot();
    await rosterRows.nth(1).getByText("预选英雄", { exact: true }).waitFor();
    assert.equal(
      await rosterRows.nth(0).locator(".participant-avatar img").count(),
      0,
      "unchanged failed image retried on unrelated snapshot",
    );
    assert.equal(
      calls.filter((c) => /Lookup|History|Profile/.test(c)).length,
      avatarCalls,
      "avatar changes requested player data",
    );
    assert(
      !(await page.locator(".player-row .participant-avatar").allTextContents())
        .join("")
        .includes("?"),
    );
    await page
      .getByRole("button", { name: "关闭玩家详情", exact: true })
      .click();
    const historyBeforeOpening = calls.filter((c) =>
      /Lookup|History/.test(c),
    ).length;
    const ensureBeforeOpening = calls.filter(
      (c) => c === "EnsureProfile",
    ).length;
    delayEnsure = true;
    await rosterRows.nth(0).click();
    for (let i = 0; !releaseEnsure && i < 200; i++)
      await page.waitForTimeout(10);
    assert(
      releaseEnsure,
      "opening roster did not request missing basic profile",
    );
    await rosterRows.nth(0).click();
    await rosterRows.nth(1).click();
    delayEnsure = false;
    releaseEnsure();
    await page
      .locator(".player-drawer")
      .getByRole("heading", { name: "未读到英雄#TEST", exact: true })
      .waitFor();
    await page.waitForFunction(() => {
      const img = document.querySelector(".player-drawer .profile-avatar img");
      return img?.complete && img.naturalWidth > 0;
    });
    assert.equal(
      calls.filter((c) => c === "EnsureProfile").length - ensureBeforeOpening,
      2,
      "same-player repeat not coalesced or latest opening lost",
    );
    assert.equal(
      calls.filter((c) => /Lookup|History/.test(c)).length,
      historyBeforeOpening,
      "opening profile reloaded history or identity",
    );
    await rosterRows.nth(1).click();
    await page
      .getByRole("button", { name: "在战绩页打开 ↗", exact: true })
      .click();
    await page
      .getByRole("heading", { name: "未读到英雄#TEST", exact: true })
      .waitFor();
    assert.equal(
      calls.filter((c) => c === "EnsureProfile").length - ensureBeforeOpening,
      2,
      "opening cached profile repeated request",
    );
    await page
      .getByRole("navigation", { name: "工作区" })
      .getByRole("button", { name: /当前对局/ })
      .click();
    await page
      .getByRole("button", { name: "关闭玩家详情", exact: true })
      .click();
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-selection.png",
      });
    snapshot = {
      ...snapshot,
      knownCount: 10,
      readyCount: 5,
      participants: Array.from({ length: 10 }, (_, i) => ({
        ...player(`快览玩家${i}#TEST`, `overlay-${i}`),
        team: i < 5 ? "ally" : "enemy",
        championId: 23,
        historyState: i < 5 ? "ready" : "pending",
        history: i < 5 ? history : [],
      })),
    };
    await page.goto("http://127.0.0.1:43199/?view=overlay");
    assert.equal(
      await page.locator(".desktop-titlebar").count(),
      0,
      "overlay keeps its own compact title bar",
    );
    await page.setViewportSize({ width: 1100, height: 640 });
    await page.getByRole("button", { name: "取消置顶", exact: true }).waitFor();
    assert.equal(
      await page
        .locator(".overlay-head")
        .evaluate((el) =>
          getComputedStyle(el).getPropertyValue("--wails-draggable").trim(),
        ),
      "drag",
    );
    assert.equal(
      await page
        .getByRole("button", { name: "关闭对局快览" })
        .evaluate((el) =>
          getComputedStyle(el).getPropertyValue("--wails-draggable").trim(),
        ),
      "no-drag",
    );
    await page.getByRole("button", { name: "取消置顶", exact: true }).click();
    await page.getByRole("button", { name: "置顶", exact: true }).waitFor();
    assert.equal(
      await page.getByRole("button", { name: /穿透|恢复鼠标/ }).count(),
      0,
    );
    const bounds = await page.locator(".overlay-footer").boundingBox();
    assert(bounds.y + bounds.height <= 640);
    const visiblePlayers = await page.locator(".overlay-player").evaluateAll(
      (rows) =>
        rows.filter((row) => {
          const r = row.getBoundingClientRect();
          return (
            r.top >= 0 &&
            r.bottom <=
              document.querySelector(".overlay-content").getBoundingClientRect()
                .bottom
          );
        }).length,
    );
    assert.equal(
      visiblePlayers,
      10,
      "normal ten-player quick view must fit without scrolling at 1100x640",
    );
    if (process.env.OVERLAY_SCREENSHOT)
      await page.screenshot({ path: process.env.OVERLAY_SCREENSHOT });
    snapshot = {
      ...snapshot,
      readyCount: 10,
      participants: snapshot.participants.map((p) => ({
        ...p,
        historyState: "ready",
        history,
      })),
    };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:snapshot",
          data: s,
        }),
      snapshot,
    );
    await page.waitForFunction(
      () =>
        document.querySelectorAll(".overlay-player .overview-metrics")
          .length === 10,
    );
    const allReady = await page.locator(".overlay-player").evaluateAll((rows) =>
      rows.every((row) => {
        const r = row.getBoundingClientRect(),
          v = row.closest(".overlay-content").getBoundingClientRect();
        return r.top >= v.top && r.bottom <= v.bottom;
      }),
    );
    assert(allReady, "all ten ready players must fit at 1100x640");
    assert.equal(
      await page
        .locator(".overlay-player .overview-metrics small")
        .filter({ hasText: /胜 \/.*负/ })
        .count(),
      10,
    );
    assert.equal(
      await page
        .locator(".overlay-player .overview-metrics small")
        .filter({ hasText: /^汇总 KDA$/ })
        .count(),
      10,
    );
    assert.equal(
      await page.locator(".overlay-player .sample-size").count(),
      10,
    );
    const nonOverlapping = await page
      .locator(".overlay-player .card-details")
      .evaluateAll((rows) =>
        rows.every((row) => {
          const metrics = row
            .querySelector(".overview-metrics")
            .getBoundingClientRect();
          const form = row
            .querySelector(".recent-block")
            .getBoundingClientRect();
          return (
            metrics.bottom <= form.top &&
            form.right <= row.getBoundingClientRect().right + 1
          );
        }),
      );
    assert(
      nonOverlapping,
      "labelled quick-view metrics and recent form must not overlap",
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-overlay.png",
      });
    await page.setViewportSize({ width: 400, height: 360 });
    const compactOverlay = await page
      .locator(".overlay-content")
      .evaluate((el) => ({
        width: el.clientWidth,
        scroll: el.scrollWidth,
        rowsFit: [
          ...el.querySelectorAll(".overlay-player .card-details"),
        ].every((row) => {
          const box = row.getBoundingClientRect();
          const metrics = row
            .querySelector(".overview-metrics")
            .getBoundingClientRect();
          const form = row
            .querySelector(".recent-block")
            .getBoundingClientRect();
          return (
            metrics.right <= box.right + 1 &&
            form.right <= box.right + 1 &&
            (form.width === 0 ||
              metrics.right + 3 <= form.left ||
              metrics.bottom <= form.top)
          );
        }),
      }));
    assert(
      compactOverlay.scroll <= compactOverlay.width + 1 &&
        compactOverlay.rowsFit,
      "minimum 400x360 quick view may scroll vertically but must not clip or overlap metrics",
    );
    const readsBeforeHold = calls.filter((c) =>
      /History|Lookup|Profile|RefreshRoster/.test(c),
    ).length;
    state = {
      ...state,
      mode: "hold",
      holdKey: "Backquote",
      revision: state.revision + 1,
    };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:overlay",
          data: s,
        }),
      state,
    );
    await page.getByText("松开 ~ 隐藏 · 鼠标操作传给游戏").waitFor();
    assert.equal(await page.locator(".overlay-controls").count(), 0);
    assert.equal(
      await page
        .locator(".overlay-shell")
        .evaluate((el) => getComputedStyle(el).pointerEvents),
      "none",
    );
    state = { ...state, mode: "persistent", revision: state.revision + 1 };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:overlay",
          data: s,
        }),
      state,
    );
    assert.equal(
      calls.filter((c) => /History|Lookup|Profile|RefreshRoster/.test(c))
        .length,
      readsBeforeHold,
      "showing overlay triggered data reads",
    );
    await page.getByRole("button", { name: "关闭对局快览" }).click();
    await page.waitForFunction(
      () => !document.querySelector(".overlay-close").disabled,
    );
    await page.keyboard.press("Escape");
    await page.waitForTimeout(100);
    assert(calls.filter((c) => c === "HideOverlay").length >= 2);
    assert.deepEqual(errors, []);
    assert.deepEqual(
      external,
      [],
      "game icons must not use remote runtime requests",
    );
    console.log(
      "PASS: 20-match rendering/density, filtering/tab state, failure retention, roster transition, local game icons/details, settings/shortcut conflict/diagnostic export, minimum layout and overlay controls (mock bridge).",
    );
  } finally {
    await browser.close();
  }
}
main().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
