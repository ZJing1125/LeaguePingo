// Validate the built development workbench with synthetic states, without Wails.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");

async function main() {
  const dist = path.resolve(__dirname, "../../artifacts/component-preview");
  const catalog = JSON.parse(
    await fs.readFile(path.join(dist, "histoire.json"), "utf8"),
  );
  assert.equal(catalog.stories.length, 5);
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.BROWSER_EXECUTABLE,
  });
  const failures = [],
    external = [];
  let checked = 0;
  async function serveAssets(route) {
    const url = new URL(route.request().url());
    if (url.origin !== "http://127.0.0.1:43200") {
      external.push(url.origin);
      return route.abort();
    }
    const file = path.resolve(
      dist,
      "." + (url.pathname === "/" ? "/index.html" : url.pathname),
    );
    assert(file.startsWith(dist + path.sep));
    const contentTypes = {
      ".js": "text/javascript",
      ".css": "text/css",
      ".json": "application/json",
      ".html": "text/html",
      ".png": "image/png",
      ".svg": "image/svg+xml",
    };
    try {
      await route.fulfill({
        body: await fs.readFile(file),
        contentType:
          contentTypes[path.extname(file)] || "application/octet-stream",
      });
    } catch {
      await route.fulfill({ status: 404, body: "Not found" });
    }
  }
  try {
    const book = await browser.newPage({
      viewport: { width: 1280, height: 800 },
    });
    book.on("pageerror", (error) =>
      failures.push(`workbench: ${error.message}`),
    );
    await book.route("**/*", serveAssets);
    await book.goto("http://127.0.0.1:43200/");
    await book.getByText("Variants", { exact: true }).waitFor();
    if (process.env.UI_SCREENSHOT_PREFIX)
      await book.screenshot({
        path: `${process.env.UI_SCREENSHOT_PREFIX}-workbench.png`,
      });
    await book.close();
    for (const story of catalog.stories) {
      for (const variant of story.variants) {
        const overlay = story.id.includes("overlayview");
        const minimum = variant.title.includes("最小尺寸");
        const narrow = variant.title.includes("窄抽屉");
        const page = await browser.newPage({
          viewport: {
            width: minimum || narrow ? 400 : overlay ? 1100 : 1280,
            height: minimum || narrow ? 360 : overlay ? 640 : 800,
          },
        });
        page.on("pageerror", (error) =>
          failures.push(`${variant.title}: ${error.message}`),
        );
        await page.route("**/*", serveAssets);
        await page.goto(
          `http://127.0.0.1:43200/__sandbox.html?storyId=${encodeURIComponent(story.id)}&variantId=${encodeURIComponent(variant.id)}`,
        );
        const selector = overlay
          ? ".overlay-shell"
          : story.id.includes("matchroster")
            ? ".team-columns"
            : story.id.includes("matchhistoryrow")
              ? ".match-card"
              : story.id.includes("profileheader")
                ? ".player-profile"
                : '[role="tablist"]';
        await page.locator(selector).waitFor({ timeout: 20000 });
        if (overlay) {
          assert.equal(await page.locator(".overlay-player").count(), 10);
          assert.equal(
            await page.evaluate(
              () => document.documentElement.scrollWidth > innerWidth,
            ),
            false,
            `${variant.title}: horizontal overflow`,
          );
        }
        if (story.id.includes("matchroster")) {
          assert.equal(await page.locator(".team-section").count(), 2);
          assert.equal(
            await page.locator(".player-row").count(),
            variant.title === "组队阶段" ? 3 : 10,
          );
          assert.equal(
            await page.evaluate(
              () => document.documentElement.scrollWidth > innerWidth,
            ),
            false,
          );
          if (variant.title === "双方十人") {
            await page.locator(".player-row").nth(5).click();
            assert.equal(await page.locator(".player-row.selected").count(), 1);
          }
        }
        if (story.id.includes("matchhistoryrow")) {
          assert.equal(
            await page.locator(".match-items .game-icon").count(),
            7,
          );
          assert.equal(
            await page.locator(".match-spells .game-icon").count(),
            2,
          );
        }
        if (narrow)
          assert.equal(
            await page.evaluate(
              () => document.documentElement.scrollWidth > innerWidth,
            ),
            false,
            "compact profile overflows",
          );
        if (story.id.includes("playertabs")) {
          await page.getByRole("tab", { selected: true }).press("ArrowRight");
          assert.equal(
            await page.getByRole("tab").nth(1).getAttribute("aria-selected"),
            "true",
          );
          for (let i = 0; i < 3; i++)
            await page.getByRole("tab", { selected: true }).press("Delete");
          await page.waitForFunction(
            () =>
              document.activeElement?.getAttribute("aria-label") ===
              "合成输入焦点回退",
          );
          assert.equal(await page.getByRole("tab").count(), 0);
        }
        if (process.env.UI_SCREENSHOT_PREFIX) {
          await page.screenshot({
            path: `${process.env.UI_SCREENSHOT_PREFIX}-${variant.id}.png`,
          });
        }
        checked++;
        await page.close();
      }
    }
    assert.equal(checked, 23);
    assert.deepEqual(failures, []);
    assert.deepEqual(
      external,
      [],
      "component examples must not contact external services",
    );
    console.log(
      `PASS: ${checked} component variants and workbench render; no Wails, external requests or page errors.`,
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
