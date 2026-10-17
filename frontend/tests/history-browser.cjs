// Explicit browsing regression with synthetic histories; no account or network access.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");

async function main() {
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.BROWSER_EXECUTABLE,
  });
  try {
    const page = await browser.newPage({
      viewport: { width: 1280, height: 800 },
    });
    const errors = [],
      external = [],
      calls = [];
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
    const history = Array.from({ length: 260 }, (_, i) => ({
      gameId: 1000 + i,
      queueId: i < 10 ? 2400 : i < 20 ? 420 : i % 2 === 0 ? 440 : 420,
      championId: 23 + (i % 3),
      playedAt: new Date(
        Date.UTC(2026, 9, 17, 0, 0) - i * 3600000,
      ).toISOString(),
      win: i % 3 === 0,
      kills: i % 11,
      deaths: 2,
      assists: 8,
      duration: 1800,
      cs: 100,
      gold: 12000,
      damage: 9999,
      items: [3006, 3078, 0, 0, 0, 0, 3364],
      spell1: 4,
      spell2: 14,
    }));
    const meta = (records) => ({
      returned: records.length,
      parsed: records.length,
      excluded: 0,
      fetchedAt: "2026-10-17T00:00:00Z",
    });
    const player = (name) => ({
      key: name,
      puuid: name,
      connectionId: 7,
      displayName: name,
      identityState: "known",
      privacyState: "unknown",
      historyState: "ready",
      history: history.slice(0, 20),
      historyMeta: meta(history.slice(0, 20)),
      stats: {},
    });
    let failNextPage = false,
      repeatNextPage = false,
      limitedNextPage = false,
      invalidNextPage = false,
      advanceBudgetNextPage = false,
      holdPage = false,
      releasePage,
      refreshStamp = 0;
    let snapshot = {
      connectionId: 7,
      generation: 1,
      connected: true,
      phase: "InProgress",
      stage: "in-game",
      frozen: true,
      participants: [],
    };
    await page.route("http://127.0.0.1:43199/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname === "/wails/custom.js")
        return route.fulfill({ contentType: "text/javascript", body: "" });
      if (url.pathname === "/wails/runtime") {
        const request = route.request().postDataJSON(),
          method = methods[request.args?.methodID],
          args = request.args?.args ?? [];
        calls.push({ method, args });
        let result;
        if (method === "Snapshot") result = snapshot;
        else if (method === "OverlayStatus")
          result = { visible: false, pinned: true, revision: 0 };
        else if (method === "PrioritizePlayer") result = null;
        else if (method === "Lookup") result = player(args[0]);
        else if (method === "RefreshHistory")
          result = {
            ...player(args[0]),
            historyMeta: {
              ...meta(history.slice(0, 20)),
              fetchedAt: new Date(
                Date.UTC(2026, 9, 17, 1) + ++refreshStamp * 1000,
              ).toISOString(),
            },
          };
        else if (method === "HistoryPage") {
          const [key, connection, offset] = args;
          assert.equal(connection, 7);
          if (holdPage) {
            holdPage = false;
            await new Promise((resolve) => {
              releasePage = resolve;
            });
          }
          if (failNextPage) {
            failNextPage = false;
            return route.fulfill({
              status: 400,
              json: {
                kind: "RuntimeError",
                message: "synthetic pagination timeout",
              },
            });
          }
          // Sparse player has no flex games within the first scan budget.
          const data =
            key === "Sparse#TEST"
              ? history.map((m) => ({ ...m, queueId: 420 }))
              : key === "Deep#TEST"
                ? history
                : history.slice(0, 140);
          const records = data.slice(offset, offset + 20);
          result = {
            matches: records,
            meta: meta(records),
            offset,
            nextOffset: offset + records.length,
            hasMore: records.length === 20,
          };
          if (repeatNextPage) {
            repeatNextPage = false;
            result.matches = history.slice(0, 20);
          }
          if (limitedNextPage) {
            limitedNextPage = false;
            result.hasMore = false;
            result.limitReason =
              "完整战绩服务暂不可用，仅能浏览有限记录；不能据此判断该模式无战绩。";
          }
          if (invalidNextPage) {
            invalidNextPage = false;
            result.nextOffset = offset;
          }
          if (advanceBudgetNextPage) {
            advanceBudgetNextPage = false;
            await page.evaluate(() => {
              window.originalDateNow = Date.now;
              Date.now = () => window.originalDateNow() + 16000;
            });
          }
        } else throw new Error(`Unexpected bridge method ${method}`);
        return route.fulfill({ json: result });
      }
      const relative =
        url.pathname === "/" ? "index.html" : url.pathname.slice(1);
      if (relative.includes("..")) return route.abort();
      return route.fulfill({
        body: await fs.readFile(path.join(__dirname, "../dist", relative)),
        contentType: relative.endsWith(".js")
          ? "text/javascript"
          : relative.endsWith(".css")
            ? "text/css"
            : relative.endsWith(".png")
              ? "image/png"
              : "text/html",
      });
    });
    const pageCalls = () => calls.filter((c) => c.method === "HistoryPage");
    async function idle() {
      await page
        .locator('[aria-label="战绩模式筛选"]:visible:enabled')
        .waitFor();
    }
    async function search(name) {
      await page.getByLabel("玩家完整 ID", { exact: true }).fill(name);
      await page.getByRole("button", { name: "查询战绩", exact: true }).click();
      await page.getByRole("heading", { name, exact: true }).waitFor();
    }
    async function select(label, value) {
      await page
        .locator(`select[aria-label="${label}"]:visible`)
        .selectOption(String(value));
      await idle();
    }
    async function held() {
      const deadline = Date.now() + 5000;
      while (!releasePage && Date.now() < deadline)
        await new Promise((resolve) => setTimeout(resolve, 10));
      assert(
        releasePage,
        "held pagination request did not arrive before the deadline",
      );
    }
    async function checkSample(queue, size) {
      const selected = history
        .slice(0, 140)
        .filter((m) => queue === "all" || m.queueId === Number(queue))
        .slice(0, size);
      assert(
        (await page.locator(".sample-scope").textContent()).includes(
          `${selected.length} / ${size} 场`,
        ),
      );
      const wins = selected.filter((m) => m.win).length;
      assert.equal(
        (await page.locator(".winrate-display > strong").textContent()).trim(),
        selected.length
          ? `${Math.round((wins * 100) / selected.length)}%`
          : "—",
      );
      const kills = selected.reduce((n, m) => n + m.kills, 0),
        assists = selected.reduce((n, m) => n + m.assists, 0),
        deaths = selected.reduce((n, m) => n + m.deaths, 0);
      assert.equal(
        (
          await page.locator(".summary-numbers dd").first().textContent()
        ).trim(),
        selected.length
          ? ((kills + assists) / Math.max(1, deaths)).toFixed(2)
          : "—",
      );
      const expected = [...new Set(selected.map((m) => m.championId))]
        .map((id) => ({
          id,
          count: selected.filter((m) => m.championId === id).length,
        }))
        .sort((a, b) => b.count - a.count);
      const visibleHeroes = await page
        .locator(".common-hero small")
        .allTextContents();
      assert.deepEqual(
        visibleHeroes.map((t) => Number(t.trim().split(" ")[0])),
        expected.map((h) => h.count),
      );
    }
    await page.goto("http://127.0.0.1:43199/");
    await search("Paged#TEST");
    assert.equal(await page.getByLabel("分析样本场数").inputValue(), "20");
    for (const [queue, label] of [
      [420, "单双排"],
      [440, "灵活排位"],
      [2400, "海克斯大乱斗"],
    ]) {
      assert.equal(
        (
          await page
            .locator(
              `select[aria-label="战绩模式筛选"] option[value="${queue}"]`,
            )
            .textContent()
        ).trim(),
        label,
      );
    }
    assert.deepEqual(
      await page
        .getByLabel("分析样本场数")
        .locator("option")
        .evaluateAll((rows) => rows.map((r) => Number(r.value))),
      [10, 20, 30, 40, 50],
    );
    assert.equal(
      pageCalls().length,
      0,
      "initial rendering must not start extra history scans",
    );
    await checkSample("all", 20);
    await select("分析样本场数", 50);
    assert.deepEqual(
      pageCalls().map((c) => c.args[2]),
      [20, 40],
    );
    await checkSample("all", 50);
    assert.equal(await page.locator(".match-card").count(), 20);
    const requestsAfter50 = pageCalls().length;
    for (const size of [10, 20, 30, 40, 50]) {
      await select("分析样本场数", size);
      await checkSample("all", size);
    }
    assert.equal(
      pageCalls().length,
      requestsAfter50,
      "cached sample choices refetched history",
    );
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    assert.equal(await page.locator(".match-card").count(), 20);
    await page.locator(".match-main").first().click();
    await page.getByText("对局 #1020", { exact: false }).waitFor();
    await checkSample("all", 50);
    await page.getByRole("button", { name: "上一页", exact: true }).click();
    assert.equal(
      pageCalls().length,
      requestsAfter50,
      "cached navigation repeated requests",
    );
    await select("战绩模式筛选", 440);
    await checkSample(440, 50);
    assert.equal(
      await page
        .locator(".match-card .match-context > strong")
        .filter({ hasText: /^灵活排位$/ })
        .count(),
      20,
    );
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 1 页"),
    );
    await select("分析样本场数", 20);
    await checkSample(440, 20);
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-analysis.png",
      });
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await search("Other#TEST");
    await page.getByRole("tab", { name: "Paged#TEST", exact: true }).click();
    assert.equal(await page.getByLabel("战绩模式筛选").inputValue(), "440");
    assert.equal(await page.getByLabel("分析样本场数").inputValue(), "20");
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 2 页"),
    );
    await checkSample(440, 20);
    await select("战绩模式筛选", 2400);
    await checkSample(2400, 20);
    await page
      .getByText("当前可查询范围内仅找到 10 场该模式记录", { exact: false })
      .waitFor();
    assert.equal(await page.locator(".match-card").count(), 10);
    assert(
      await page
        .getByRole("button", { name: "下一页", exact: true })
        .isDisabled(),
    );
    await select("战绩模式筛选", 450);
    await checkSample(450, 20);
    assert.equal(await page.locator(".match-card").count(), 0);
    await select("战绩模式筛选", "all");
    await select("分析样本场数", 50);
    const exhaustedRequests = pageCalls().length;
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    assert.equal(
      pageCalls().length,
      exhaustedRequests,
      "refresh started unrequested scans",
    );
    assert(
      (await page.locator(".sample-scope").textContent()).includes("20 / 50"),
    );
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 1 页"),
    );
    await page.getByRole("button", { name: "补齐样本", exact: true }).waitFor();
    await page.getByRole("button", { name: "补齐样本", exact: true }).click();
    await idle();
    await checkSample("all", 50);
    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    failNextPage = true;
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    await idle();
    await page
      .getByText("synthetic pagination timeout", { exact: false })
      .waitFor();
    assert.equal(
      await page.locator(".match-card").count(),
      20,
      "failed navigation discarded visible records",
    );
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 1 页"),
    );
    await page.getByRole("button", { name: "重试", exact: true }).click();
    await idle();
    await checkSample("all", 50);
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 2 页"),
    );

    await search("Sparse#TEST");
    const beforeSparse = pageCalls().length;
    await select("战绩模式筛选", 440);
    assert.equal(
      pageCalls().length - beforeSparse,
      5,
      "rare mode exceeded five-request scan budget",
    );
    await page.getByRole("button", { name: "继续查找", exact: true }).waitFor();
    await page.getByRole("button", { name: "继续查找", exact: true }).click();
    await idle();
    assert.equal(
      pageCalls().length - beforeSparse,
      10,
      "continuation did not preserve its cursor and budget",
    );
    const sparseOffsets = pageCalls()
      .slice(beforeSparse)
      .map((c) => c.args[2]);
    assert.deepEqual(
      sparseOffsets,
      [20, 40, 60, 80, 100, 120, 140, 160, 180, 200],
    );
    await page.setViewportSize({ width: 880, height: 620 });
    assert.equal(
      await page.evaluate(
        () => document.documentElement.scrollWidth > innerWidth,
      ),
      false,
    );
    assert(await page.getByLabel("分析样本场数").isVisible());
    const pagerBounds = await page
      .getByRole("navigation", { name: "战绩分页" })
      .boundingBox();
    assert(
      pagerBounds.y + pagerBounds.height <= 620,
      "pagination controls below the supported viewport",
    );

    // A held request can finish after switching players, but cannot continue
    // scanning or alter the new player's mode, sample, page or data.
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.getByRole("tab", { name: "Other#TEST", exact: true }).click();
    holdPage = true;
    await page.getByLabel("分析样本场数").selectOption("50");
    await page.waitForFunction(() =>
      document
        .querySelector(".browse-status")
        ?.textContent.includes("正在查找"),
    );
    await held();
    const beforeSwitch = pageCalls().length;
    await page.getByRole("tab", { name: "Sparse#TEST", exact: true }).click();
    releasePage();
    await idle();
    assert.equal(
      pageCalls().length,
      beforeSwitch,
      "switching players left a background scan running",
    );
    assert.equal(await page.getByLabel("战绩模式筛选").inputValue(), "440");
    assert.equal(
      await page.locator(".match-card").count(),
      0,
      "old page leaked into new player",
    );
    await page.getByRole("tab", { name: "Other#TEST", exact: true }).click();
    assert(
      (await page.locator(".sample-scope").textContent()).includes("40 / 50"),
    );
    await page.getByRole("button", { name: "继续查找", exact: true }).click();
    await idle();
    await checkSample("all", 50);

    // Stopping and elapsed-time limits must both prevent a second request.
    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    await select("分析样本场数", 10);
    holdPage = true;
    releasePage = undefined;
    await page.getByLabel("分析样本场数").selectOption("50");
    await held();
    const stoppedCalls = pageCalls().length;
    await page.getByRole("button", { name: "停止查找", exact: true }).click();
    releasePage();
    await idle();
    assert.equal(
      pageCalls().length,
      stoppedCalls,
      "stop started another source request",
    );
    assert(
      (await page.locator(".sample-scope").textContent()).includes("40 / 50"),
    );
    await page
      .getByText("已停止查找，已获取记录保留。", { exact: true })
      .waitFor();

    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    advanceBudgetNextPage = true;
    const beforeElapsed = pageCalls().length;
    await page.getByRole("button", { name: "补齐样本", exact: true }).click();
    await idle();
    await page.evaluate(() => {
      Date.now = window.originalDateNow;
    });
    assert.equal(
      pageCalls().length - beforeElapsed,
      1,
      "expired time budget started another source request",
    );
    await page.getByRole("button", { name: "继续查找", exact: true }).click();
    await idle();
    await checkSample("all", 50);

    // Invalid cursors must not advance or discard the last valid page.
    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    invalidNextPage = true;
    const beforeInvalid = pageCalls().length;
    await page.getByRole("button", { name: "补齐样本", exact: true }).click();
    await idle();
    await page.getByText("战绩分页位置无效", { exact: false }).waitFor();
    assert.equal(await page.locator(".match-card").count(), 20);
    await page.getByRole("button", { name: "重试", exact: true }).click();
    await idle();
    assert.deepEqual(
      pageCalls()
        .slice(beforeInvalid)
        .map((c) => c.args[2]),
      [20, 20, 40],
    );
    await checkSample("all", 50);

    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    repeatNextPage = true;
    const beforeRepeat = pageCalls().length;
    await page.getByRole("button", { name: "补齐样本", exact: true }).click();
    await idle();
    assert.equal(
      pageCalls().length - beforeRepeat,
      1,
      "duplicate first page triggered more requests",
    );
    await page
      .getByText("数据源重复返回已获取的战绩", { exact: false })
      .waitFor();
    assert.equal(await page.locator(".match-card").count(), 20);
    assert.equal(
      await page.locator(".quality-warning").count(),
      0,
      "rejected repeated pages inflated exclusion counts",
    );
    await page.getByRole("button", { name: "重试", exact: true }).click();
    await idle();
    await checkSample("all", 50);

    await page.getByRole("button", { name: "刷新战绩", exact: true }).click();
    await idle();
    limitedNextPage = true;
    await select("战绩模式筛选", 450);
    await page.getByText("尚未取得该模式的战绩", { exact: true }).waitFor();
    assert(
      (await page.locator(".browse-status").textContent()).includes(
        "不能据此判断",
      ),
    );
    assert(
      !(await page.locator(".browse-status").textContent()).includes(
        "仅找到 0 场",
      ),
    );
    assert(
      !(await page.locator(".page-indicator").textContent()).includes(
        "共 1 页",
      ),
    );

    // Jump supports cached and unqueried pages; invalid input never sends requests.
    await search("Jump#TEST");
    const jumpInput = page.getByLabel("跳转页码", { exact: true });
    const jumpButton = page.getByRole("button", { name: "跳转", exact: true });
    const beforeBadJump = pageCalls().length;
    for (const input of ["0", "-1", "1.5", "abc", "51", ""]) {
      await jumpInput.fill(input);
      await jumpButton.click();
      await page
        .getByRole("alert")
        .filter({ hasText: "请输入 1–50 的整数页码" })
        .waitFor();
    }
    assert.equal(
      pageCalls().length,
      beforeBadJump,
      "invalid jump sent history requests",
    );
    await jumpInput.fill("3");
    await jumpInput.press("Enter");
    await idle();
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 3 页"),
    );
    assert.equal(await jumpInput.inputValue(), "3");
    assert.deepEqual(
      pageCalls()
        .slice(beforeBadJump)
        .map((c) => c.args[2]),
      [20, 40],
    );
    await checkSample("all", 20);
    const beforeCachedJump = pageCalls().length;
    await jumpInput.fill("2");
    await jumpButton.click();
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 2 页"),
    );
    assert.equal(pageCalls().length, beforeCachedJump);
    await jumpInput.fill("7");
    await jumpButton.click();
    await idle();
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 7 页"),
    );
    await jumpInput.fill("8");
    await jumpButton.click();
    await idle();
    await page
      .getByText("当前可查询范围内没有第 8 页，已保留当前页。", { exact: true })
      .waitFor();
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 7 页"),
    );
    const beforeOutOfRange = pageCalls().length;
    await jumpButton.click();
    await page
      .getByRole("alert")
      .filter({ hasText: "请输入 1–7 的整数页码" })
      .waitFor();
    assert.equal(pageCalls().length, beforeOutOfRange);
    await select("战绩模式筛选", 440);
    assert.equal(
      await jumpInput.inputValue(),
      "1",
      "mode change retained the old page draft",
    );
    await jumpInput.fill("3");
    await jumpInput.press("Enter");
    await idle();
    await checkSample(440, 20);
    assert.equal(await page.locator(".match-card").count(), 20);
    assert.equal(
      await page
        .locator(".match-context > strong")
        .filter({ hasText: /^灵活排位$/ })
        .count(),
      20,
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-jump.png",
      });
    await page.setViewportSize({ width: 880, height: 620 });
    const bounds = await page
      .getByRole("navigation", { name: "战绩分页" })
      .boundingBox();
    assert(
      bounds.y + bounds.height < 620,
      "embedded pagination must be inset from the window edge",
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-minimum.png",
      });
    await page.setViewportSize({ width: 1280, height: 800 });

    // A far jump retains its target across bounded scans instead of issuing
    // an unbounded request chain or silently switching to a different page.
    await search("Deep#TEST");
    const beforeDeepJump = pageCalls().length;
    await jumpInput.fill("10");
    await jumpInput.press("Enter");
    await idle();
    assert.equal(pageCalls().length - beforeDeepJump, 5);
    assert(
      (await page.locator(".page-indicator").textContent()).includes("第 1 页"),
    );
    assert.equal(await jumpInput.inputValue(), "10");
    await page
      .getByText("本轮查找已暂停，已获取记录保留；可继续查找第 10 页。", {
        exact: true,
      })
      .waitFor();
    await page.getByRole("button", { name: "继续查找", exact: true }).click();
    await idle();
    assert.equal(pageCalls().length - beforeDeepJump, 9);
    assert.deepEqual(
      pageCalls()
        .slice(beforeDeepJump)
        .map((c) => c.args[2]),
      [20, 40, 60, 80, 100, 120, 140, 160, 180],
    );
    assert(
      (await page.locator(".page-indicator").textContent()).includes(
        "第 10 页",
      ),
    );
    assert.equal(await jumpInput.inputValue(), "10");
    assert.equal(await page.locator(".match-card").count(), 20);
    await checkSample("all", 20);
    await page.getByRole("tab", { name: "Other#TEST", exact: true }).click();

    // Disconnected criteria changes may read cache but must not fetch history.
    snapshot = { ...snapshot, generation: 2, connected: false };
    await page.evaluate(
      (s) =>
        window._wails.dispatchWailsEvent({
          name: "leaguepingo:snapshot",
          data: s,
        }),
      snapshot,
    );
    const beforeOffline = pageCalls().length;
    await select("战绩模式筛选", 440);
    assert.equal(pageCalls().length, beforeOffline);
    assert.equal(
      await page.getByRole("button", { name: "补齐样本", exact: true }).count(),
      0,
    );

    snapshot = {
      ...snapshot,
      generation: 3,
      connected: true,
      participants: [
        { ...player("Drawer#TEST"), team: "ally", championId: 23 },
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
    await page
      .getByRole("navigation", { name: "工作区" })
      .getByRole("button", { name: /当前对局/ })
      .click();
    await page.locator(".player-row").first().click();
    await page.setViewportSize({ width: 880, height: 620 });
    const drawer = page.locator(".player-drawer");
    await drawer
      .getByRole("heading", { name: "Drawer#TEST", exact: true })
      .waitFor();
    await select("分析样本场数", 50);
    assert.equal(await drawer.locator(".match-card").count(), 20);
    await drawer.getByRole("button", { name: "下一页", exact: true }).click();
    assert(
      (await drawer.locator(".page-indicator").textContent()).includes(
        "第 2 页",
      ),
    );
    const compactBounds = await drawer
      .getByRole("navigation", { name: "战绩分页" })
      .boundingBox();
    assert(
      compactBounds.y + compactBounds.height <= 620,
      "compact pagination exceeds minimum height",
    );
    await drawer.getByLabel("跳转页码", { exact: true }).fill("1");
    await drawer.getByRole("button", { name: "跳转", exact: true }).click();
    assert(
      (await drawer.locator(".page-indicator").textContent()).includes(
        "第 1 页",
      ),
    );
    assert(
      await drawer
        .getByRole("button", { name: "跳转", exact: true })
        .isVisible(),
    );
    assert.equal(
      await drawer.evaluate((el) => el.scrollWidth > el.clientWidth),
      false,
      "compact history overflows horizontally",
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-compact.png",
      });
    assert.deepEqual(errors, []);
    assert.deepEqual(external, []);
    console.log(
      "PASS: mode-linked 10–50 samples, embedded pagination/jump/cache, bounded far-jump continuation, independent refresh, terminal/empty/limited modes, duplicate guards, failed/invalid-page retry, request/time/stop budgets, player isolation, offline cache and compact/minimum layout.",
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
