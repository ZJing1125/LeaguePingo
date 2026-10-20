// Synthetic runtime: comparison cards, honest sample rings and stable details.
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
      viewport: { width: 1280, height: 800 },
    });
    page.setDefaultTimeout(15000);
    const errors = [],
      external = [],
      calls = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const desktop = mockWindowRuntime(page);
    const binding = await fs.readFile(
      path.join(__dirname, "../bindings/leaguepingo/appservice.ts"),
      "utf8",
    );
    const methods = Object.fromEntries(
      [
        ...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g),
      ].map((m) => [m[2], m[1]]),
    );
    const avatar =
      "data:image/svg+xml;base64," +
      Buffer.from(
        '<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><rect width="40" height="40" rx="5" fill="#397d78"/><circle cx="20" cy="20" r="9" fill="#f2c284"/></svg>',
      ).toString("base64");
    const history = Array.from({ length: 20 }, (_, i) => ({
      gameId: 90000 + i,
      playedAt: "2026-10-19T01:00:00Z",
      queueId: 420,
      championId: 107,
      win: i % 2 === 0,
      kills: 6,
      deaths: 3,
      assists: 9,
      duration: 1800,
      items: [3006, 3078, 0, 0, 0, 0, 3364],
      spell1: 4,
      spell2: 14,
    }));
    function profile(index = 0) {
      return {
        updatedAt: "2026-10-19T01:00:00Z",
        profileIconData: avatar,
        level: 100,
        privacyState: "private",
        rankState: "ready",
        ranks: [
          { queue: "RANKED_SOLO_5x5", tier: ["GOLD", "DIAMOND", "EMERALD", "MASTER", "PLATINUM"][index % 5], division: "II", lp: 52, wins: 154 + index * 7, losses: 138 + index * 3 },
        ],
        fields: { rank: { state: "ready", fetchedAt: "2026-10-19T01:00:00Z" } },
      };
    }
    const players = Array.from({ length: 10 }, (_, i) => ({
      connectionId: 1,
      key: `synthetic-${i}`,
      puuid: `synthetic-${i}`,
      displayName: `合成玩家${i + 1}#DEMO`,
      team: i < 5 ? "ally" : "enemy",
      championId: [107, 23, 25][i % 3],
      championState: "selected",
      identityState: "known",
      privacyState: "private",
      historyState: "ready",
      history: history.map((m, j) => ({ ...m, position: ["TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"][i % 5], mapId: 11, playedAt: new Date(Date.parse("2026-10-19T01:00:00Z") - j * 7200000).toISOString(), cs: 180 + i * 5, win: (j + i) % (i % 2 ? 3 : 2) !== 0 })),
      historyMeta: {
        fetchedAt: "2026-10-19T01:00:00Z",
        returned: 20,
        parsed: 20,
      },
      stats: { games: 20, winRate: 50, kda: 5 },
      profile: profile(i),
    }));
    let snapshot = {
      connectionId: 1,
      generation: 1,
      gameId: 42,
      connected: true,
      stage: "preparing",
      phase: "ChampSelect",
      frozen: false,
      knownCount: 10,
      readyCount: 10,
      participants: structuredClone(players),
    };
    let refreshes = 0;
    await page.route("**/*", (route) => {
      external.push(route.request().url());
      return route.abort();
    });
    await page.route("http://127.0.0.1:43201/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname === "/wails/custom.js")
        return route.fulfill({ contentType: "text/javascript", body: "" });
      if (url.pathname === "/wails/runtime") {
        const request = route.request().postDataJSON();
        if (await desktop.handle(route, request)) return;
        const method = methods[request.args?.methodID],
          args = request.args?.args ?? [];
        calls.push(method);
        let result = null;
        if (method === "Snapshot") result = snapshot;
        else if (method === "OverlayStatus")
          result = { visible: false, pinned: true, revision: 0 };
        else if (method === "RefreshData") {
          assert.equal(args[1], 1);
          refreshes++;
          const original = snapshot.participants.find((p) => p.key === args[0]);
          assert(original);
          result = {
            ...original,
            profile: {
              ...original.profile,
              updatedAt: "2026-10-20T01:00:00Z",
              rankMessage: "段位查询超时，可重试",
              fields: {
                ...original.profile.fields,
                rank: { state: "retained", fetchedAt: "2026-10-19T01:00:00Z" },
              },
            },
          };
        } else if (method !== "PrioritizePlayer")
          throw new Error(`Unexpected request: ${method}`);
        return route.fulfill({ json: result });
      }
      const relative =
        url.pathname === "/" ? "index.html" : url.pathname.slice(1);
      if (relative.includes("..")) return route.abort();
      const ext = path.extname(relative);
      return route.fulfill({
        body: await fs.readFile(path.join(__dirname, "../dist", relative)),
        contentType:
          {
            ".js": "text/javascript",
            ".css": "text/css",
            ".png": "image/png",
            ".jpg": "image/jpeg",
          }[ext] ?? "text/html",
      });
    });
    async function screenshot(name) {
      if (process.env.UI_SCREENSHOT_PREFIX)
        await page.screenshot({
          path: `${process.env.UI_SCREENSHOT_PREFIX}-${name}.png`,
        });
    }
    async function publish() {
      snapshot = { ...snapshot, generation: snapshot.generation + 1 };
      await page.evaluate(
        (s) =>
          window._wails.dispatchWailsEvent({
            name: "leaguepingo:snapshot",
            data: s,
          }),
        snapshot,
      );
    }
    async function sizes() {
      return page
        .locator(".player-row")
        .evaluateAll((rows) =>
          rows.map((el) => ({
            width: el.getBoundingClientRect().width,
            height: el.getBoundingClientRect().height,
            recent: getComputedStyle(el.querySelector(".recent-block")).display,
          })),
        );
    }
    async function noOverflow() {
      assert(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      );
    }
    const rows = page.locator(".player-row"),
      drawer = page.locator(".player-drawer");
    await page.goto("http://127.0.0.1:43201/");
    await page.getByRole("button", { name: "当前对局", exact: true }).click();
    await rows.last().waitFor();
    assert.equal(await rows.count(), 10);
    assert(
      await rows
        .first()
        .evaluate(
          (el) =>
            parseFloat(
              getComputedStyle(el.querySelector(".scout-ring b"))
                .fontSize,
            ) >= 14,
        ),
    );
    assert(
      await rows
        .first()
        .evaluate(
          (el) =>
            parseFloat(
              getComputedStyle(el.querySelector(".rank-line")).fontSize,
            ) >= 11,
        ),
    );
    await noOverflow();
    assert.equal(await page.locator(".scout-rings").count(), 10);
    assert.equal(await page.locator(".scout-ring-metric").count(), 30);
    assert.equal(await page.locator(".card-artwork").count(), 0, "comparison cards use flat, aligned data cells");
    assert((await rows.first().locator(".scout-rings").innerText()).includes("上路"));
    assert((await rows.first().locator(".scout-scope").innerText()).includes("20 场内"));
    assert(await rows.evaluateAll(cards => cards.every(card => card.getBoundingClientRect().bottom <= innerHeight)), "all ten comparison cards should fit at 1280x800");
    await screenshot("cards");
    await page.getByRole("combobox", { name: "界面主题", exact: true }).selectOption("light");
    await screenshot("cards-light");
    await page.getByRole("combobox", { name: "界面主题", exact: true }).selectOption("dark");
    const before = await sizes();
    await rows.first().click();
    await drawer.waitFor();
    assert.equal(await drawer.getAttribute("role"), "dialog");
    assert.deepEqual(
      await sizes(),
      before,
      "opening details must not shrink cards or hide information",
    );
    assert.equal(await page.locator(".app-main").getAttribute("inert"), "");
    assert.equal(
      await drawer
        .getByRole("button", { name: "刷新数据", exact: true })
        .count(),
      1,
    );
    assert.equal(
      await page
        .getByRole("button", {
          name: /获取段位|更新玩家资料|刷新战绩|重试段位/,
        })
        .count(),
      0,
      "only one data refresh control",
    );
    await drawer.getByText("生涯不公开", { exact: true }).waitFor();
    assert(!(await drawer.textContent()).includes("客户端明确返回"));
    assert.equal(await drawer.locator(".rank-card").count(), 2);
    await drawer.getByLabel("分析样本场数").selectOption("10");
    await drawer.getByLabel("战绩模式筛选").selectOption("420");
    await drawer.getByRole("button", { name: "刷新数据", exact: true }).click();
    await drawer.getByText("旧段位", { exact: true }).waitFor();
    assert.equal(refreshes, 1);
    assert.equal(
      await page.locator(".player-tab").count(),
      0,
      "refresh must not open history tabs",
    );
    assert.equal(await drawer.getByLabel("分析样本场数").inputValue(), "10");
    assert.equal(await drawer.getByLabel("战绩模式筛选").inputValue(), "420");
    await screenshot("unified-details");
    await drawer
      .getByRole("button", { name: "下一位玩家", exact: true })
      .click();
    await drawer
      .getByRole("heading", { name: "合成玩家2#DEMO", exact: true })
      .waitFor();
    await drawer
      .getByRole("button", { name: "上一位玩家", exact: true })
      .click();
    await drawer
      .getByRole("heading", { name: "合成玩家1#DEMO", exact: true })
      .waitFor();
    assert.equal(await drawer.getByLabel("分析样本场数").inputValue(), "10");
    await page.setViewportSize({ width: 1100, height: 760 });
    assert.equal((await sizes())[0].height, before[0].height);
    await page.setViewportSize({ width: 880, height: 620 });
    await noOverflow();
    assert(
      await drawer.evaluate((el) => {
        const r = el.getBoundingClientRect();
        return (
          r.left >= 0 &&
          r.right <= innerWidth &&
          r.top >= 36 &&
          r.bottom <= innerHeight &&
          el.scrollWidth <= el.clientWidth
        );
      }),
    );
    await drawer.focus();
    await page.keyboard.press("Shift+Tab");
    assert(await drawer.evaluate((el) => el.contains(document.activeElement)));
    await page.keyboard.press("Tab");
    await page.waitForFunction(() =>
      document.activeElement?.textContent.includes("在战绩页打开"),
    );
    await screenshot("details-880");
    await page.keyboard.press("Escape");
    await drawer.waitFor({ state: "detached" });
    await page.waitForFunction(
      () => document.activeElement?.dataset.playerKey === "synthetic-0",
    );
    assert.equal(await page.locator(".app-main").getAttribute("inert"), null);
    await page.setViewportSize({ width: 1100, height: 820 });
    snapshot = {
      ...snapshot,
      stage: "party",
      phase: "Lobby",
      gameId: 0,
      knownCount: 1,
      readyCount: 1,
      participants: [
        {
          ...structuredClone(players[0]),
          championId: 0,
          championState: "unselected",
        },
      ],
    };
    await publish();
    await rows.first().getByText("等级 100", { exact: true }).waitFor();
    assert.equal(await rows.count(), 1);
    assert(!(await rows.first().textContent()).includes("英雄信息暂不可用"));
    await page.waitForFunction(
      () =>
        document.querySelector(".player-row .participant-avatar img")
          ?.naturalWidth > 0,
    );
    await rows.first().getByText("黄金 II · 52 LP", { exact: true }).waitFor();
    const lobbyBefore = await sizes();
    await screenshot("lobby");
    await rows.first().click();
    await drawer.waitFor();
    assert.deepEqual(
      await sizes(),
      lobbyBefore,
      "lobby card must remain unchanged",
    );
    await page.keyboard.press("Escape");
    await drawer.waitFor({ state: "detached" });
    snapshot = {
      ...snapshot,
      stage: "preparing",
      phase: "ChampSelect",
      gameId: 43,
      participants: [
        structuredClone(players[0]),
        {
          ...structuredClone(players[5]),
          identityState: "unavailable",
          displayName: "",
          championId: 0,
          championState: "unselected",
          history: [],
          historyState: "pending",
          profile: undefined,
        },
      ],
    };
    await publish();
    await rows.first().locator(".card-pick .game-icon").waitFor();
    assert(
      await rows.first().evaluate((el) => {
        const hero = el.querySelector(".scout-champion").getBoundingClientRect(),
          rank = el.querySelector(".scout-rank").getBoundingClientRect();
        return (
          hero.right <= rank.left + 1 && Math.abs(hero.top - rank.top) < 1 &&
          getComputedStyle(el).clipPath === "none"
        );
      }),
      "hero sample and rank must use aligned cells without clipping or a portrait strip",
    );
    await screenshot("champ-select");
    snapshot = {
      ...snapshot,
      knownCount: 12,
      readyCount: 12,
      participants: Array.from({ length: 12 }, (_, i) => ({
        ...structuredClone(players[0]),
        key: `large-${i}`,
        puuid: `large-${i}`,
        team: i < 6 ? "ally" : "enemy",
      })),
    };
    await publish();
    await rows.last().waitFor();
    assert.equal(await rows.count(), 12);
    assert(
      await page.locator(".team-columns").evaluate((el) => {
        const t = el.querySelectorAll(".team-section");
        return (
          t[0].querySelector(".player-row:last-child").getBoundingClientRect()
            .bottom <= t[1].getBoundingClientRect().top
        );
      }),
    );
    await noOverflow();
    assert.equal(calls.filter((c) => c === "RefreshData").length, 1);
    snapshot.participants = [{ ...structuredClone(players[0]), history: history.map(m => ({ ...m, win: null })), profile: { ...profile(), rankState: "unavailable", ranks: [] } }];
    await publish();
    await rows.first().locator(".scout-ring-metric").first().getByText("胜负暂缺", { exact: true }).waitFor();
    assert((await rows.first().locator(".scout-ring-metric").first().innerText()).includes("—"));
    await rows.first().getByText("来源未提供段位", { exact: true }).waitFor();
    await screenshot("partial");
    assert(
      !calls.some((c) =>
        /LoadRanks|LoadProfile|LookupPlayer|HistoryPage|EnsureProfile/.test(c),
      ),
      "warm layout/selection must not start redundant data queries",
    );
    assert.deepEqual(errors, []);
    assert.deepEqual(external, []);
    console.log(
      "PASS: ten comparison cards, hero/rank cells, sample rings/roles/tags, both themes, full lobby profile, unified refresh, no extra queries, stable modal geometry/focus and expanded rosters.",
    );
  } finally {
    await browser.close();
  }
}
main().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
