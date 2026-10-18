// Synthetic runtime only: geometry and focus behavior, no client/network data.
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
    const history = Array.from({ length: 20 }, (_, i) => ({
      gameId: 90000 + i,
      playedAt: "2026-10-18T01:00:00Z",
      queueId: 420,
      championId: 23,
      win: i % 2 === 0,
      kills: 6,
      deaths: 3,
      assists: 9,
      duration: 1800,
      items: [3006, 3078, 0, 0, 0, 0, 3364],
      spell1: 4,
      spell2: 14,
    }));
    let snapshot = {
      connectionId: 1,
      generation: 1,
      gameId: 42,
      connected: true,
      stage: "in-game",
      phase: "InProgress",
      frozen: true,
      knownCount: 10,
      readyCount: 10,
      message:
        "本局开局资料补齐已结束；未完成的玩家可手动查询，本局不再自动重试。",
      participants: Array.from({ length: 10 }, (_, i) => ({
        connectionId: 1,
        key: `synthetic-${i}`,
        displayName:
          i === 1
            ? "用于检查省略显示的中英文长玩家名称LongName#DEMO"
            : `合成玩家${i + 1}#DEMO`,
        team: i < 5 ? "ally" : "enemy",
        championId: [23, 24, 25][i % 3],
        championState: "selected",
        identityState: "known",
        privacyState: "unknown",
        historyState: "ready",
        history,
        historyMeta: {
          fetchedAt: "2026-10-18T01:00:00Z",
          returned: 20,
          parsed: 20,
        },
        stats: { games: 20, winRate: 50, kda: 5 },
        profile: {
          level: 100,
          profileIconData:
            "data:image/svg+xml;base64," +
            Buffer.from(
              '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" rx="6" fill="#305b53"/><circle cx="16" cy="16" r="7" fill="#62d6ba"/></svg>',
            ).toString("base64"),
          rankState: "unavailable",
          ranks: [],
          privacyState: "unknown",
          updatedAt: "2026-10-18T01:00:00Z",
        },
      })),
    };
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
        const method = methods[request.args?.methodID];
        calls.push(method);
        let result = null;
        if (method === "Snapshot") result = snapshot;
        else if (method === "OverlayStatus")
          result = { visible: false, pinned: true, revision: 0 };
        else if (method !== "PrioritizePlayer")
          throw new Error(`Unexpected request: ${method}`);
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
    async function screenshot(name) {
      if (process.env.UI_SCREENSHOT_PREFIX)
        await page.screenshot({
          path: `${process.env.UI_SCREENSHOT_PREFIX}-${name}.png`,
        });
    }
    async function geometry(allVisible = true) {
      const result = await page.evaluate(() => {
        const teams = [...document.querySelectorAll(".team-section")].map((e) =>
          e.getBoundingClientRect(),
        );
        const rows = [...document.querySelectorAll(".player-row")].map((e) =>
          e.getBoundingClientRect(),
        );
        const list = document
          .querySelector(".team-columns")
          .getBoundingClientRect();
        return {
          paired:
            teams[0].right <= teams[1].left &&
            Math.abs(teams[0].top - teams[1].top) < 1,
          count: rows.length,
          visible: rows.every(
            (r) =>
              r.top >= list.top &&
              r.bottom <= Math.min(list.bottom, innerHeight),
          ),
          overflow: document.documentElement.scrollWidth > innerWidth,
          inside: rows.every(
            (r) => r.left >= list.left && r.right <= list.right,
          ),
        };
      });
      assert.equal(result.count, 10);
      assert(result.paired, "both teams must remain side by side");
      if (allVisible)
        assert(
          result.visible,
          "all ten players must fit in the visible roster",
        );
      assert(
        !result.overflow && result.inside,
        "roster must not overflow horizontally",
      );
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
    await page.goto("http://127.0.0.1:43201/");
    await page.getByRole("button", { name: "当前对局", exact: true }).click();
    await page.locator(".player-row").last().waitFor();
    await geometry();
    await screenshot("roster");
    const rows = page.locator(".player-row"),
      drawer = page.locator(".player-drawer");
    await rows.nth(1).click();
    await drawer.waitFor();
    assert.equal(await drawer.getAttribute("role"), "complementary");
    await geometry();
    await rows.nth(7).click();
    await drawer.getByRole("heading", { name: "合成玩家8#DEMO" }).waitFor();
    assert.equal(await rows.nth(7).getAttribute("aria-pressed"), "true");
    await drawer.getByLabel("分析样本场数").selectOption("10");
    await drawer.getByLabel("战绩模式筛选").selectOption("420");
    await screenshot("roster-inline");
    // Expanded navigation uses the available main width, preserving both teams.
    await page.getByRole("button", { name: "展开导航" }).click();
    await geometry();
    await screenshot("roster-expanded-nav");
    await page.getByRole("button", { name: "收起导航" }).click();
    await page.setViewportSize({ width: 880, height: 620 });
    await page.getByRole("dialog", { name: "玩家详情" }).waitFor();
    assert.equal(await drawer.getByLabel("分析样本场数").inputValue(), "10");
    assert.equal(await drawer.getByLabel("战绩模式筛选").inputValue(), "420");
    await page.waitForFunction(() =>
      document.querySelector(".player-drawer").contains(document.activeElement),
    );
    assert.equal(await page.locator(".app-main").getAttribute("inert"), "");
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
    await geometry(false);
    // Tab wraps in both directions and ignores hidden/disabled controls.
    await drawer.focus();
    await page.keyboard.press("Shift+Tab");
    assert(await drawer.evaluate((el) => el.contains(document.activeElement)));
    await page.keyboard.press("Tab");
    await page.waitForFunction(() =>
      document.activeElement?.textContent.includes("在战绩页打开"),
    );
    const firstFocused = await page.evaluate(
      () => document.activeElement.outerHTML,
    );
    await publish();
    assert.equal(
      await page.evaluate(() => document.activeElement.outerHTML),
      firstFocused,
      "snapshots must not steal focus",
    );
    await screenshot("roster-modal");
    await page.keyboard.press("Escape");
    await drawer.waitFor({ state: "detached" });
    assert.equal(await page.locator(".app-main").getAttribute("inert"), null);
    await page.waitForFunction(
      () => document.activeElement?.dataset.playerKey === "synthetic-7",
    );
    await rows.nth(0).click();
    await page.getByRole("dialog", { name: "玩家详情" }).waitFor();
    await page
      .locator(".drawer-floating")
      .click({ position: { x: 12, y: 40 } });
    await drawer.waitFor({ state: "detached" });
    await page.waitForFunction(
      () => document.activeElement?.dataset.playerKey === "synthetic-0",
    );
    await rows.nth(9).click();
    await page.getByRole("button", { name: "在战绩页打开 ↗" }).click();
    await page.waitForFunction(
      () => document.activeElement?.getAttribute("role") === "tab",
    );
    assert.equal(
      await page.getByRole("dialog", { name: "玩家详情" }).count(),
      0,
    );
    await page.getByRole("button", { name: "当前对局", exact: true }).click();
    await page.getByRole("dialog", { name: "玩家详情" }).waitFor();
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.waitForFunction(
      () =>
        document.querySelector(".player-drawer")?.getAttribute("role") ===
        "complementary",
    );
    await geometry();
    // Failed/hidden/loading/empty retain distinct wording and still fit in one screen.
    snapshot.participants = snapshot.participants.map((p, i) =>
      i < 5
        ? p
        : {
            ...p,
            historyState:
              i === 9
                ? "pending"
                : i === 8
                  ? "empty"
                  : i === 7
                    ? "failed"
                    : "loading",
            identityState: i === 9 ? "unavailable" : "known",
            history: [],
          },
    );
    snapshot.knownCount = 9;
    await publish();
    await page.getByRole("button", { name: "关闭玩家详情" }).click();
    await geometry();
    assert.equal(await page.getByText("正在获取", { exact: true }).count(), 2);
    assert.equal(await page.getByText("可重试", { exact: true }).count(), 1);
    assert.equal(await page.getByText("身份未知", { exact: true }).count(), 1);
    assert(
      (await page.getByLabel("本局资料准备进度").textContent()).includes(
        "6/ 10",
      ),
    );
    await screenshot("roster-partial");
    // Hero notes share the second row, including unselected and intent states.
    snapshot.participants = snapshot.participants.map((p, i) => ({
      ...p,
      championId: i % 2 ? 0 : 23,
      championState: i % 2 ? "unselected" : "intent",
    }));
    await publish();
    await geometry();
    await rows.nth(0).click();
    await geometry();
    await page.getByRole("button", { name: "关闭玩家详情" }).click();
    // If the selected row disappears, return to an available match control.
    await rows.nth(1).click();
    snapshot.participants = snapshot.participants.filter(
      (p) => p.key !== "synthetic-1",
    );
    await publish();
    await drawer.waitFor({ state: "detached" });
    await page.waitForFunction(
      () =>
        document.activeElement ===
        document.querySelector(".roster-toolbar button"),
    );

    assert(
      !calls.includes("EnsureProfile") &&
        !calls.includes("LookupPlayer") &&
        !calls.includes("HistoryPage"),
      "layout and snapshots cannot trigger extra data queries",
    );
    assert.deepEqual(errors, []);
    assert.deepEqual(external, []);
    console.log(
      "PASS: 1280×800 ten-player comparison with inline details/expanded navigation, 880×620 modal geometry, Tab/Esc/backdrop/focus return, resize continuity and partial states without extra requests.",
    );
  } finally {
    await browser.close();
  }
}
main().catch((e) => {
  console.error(e);
  process.exitCode = 1;
});
