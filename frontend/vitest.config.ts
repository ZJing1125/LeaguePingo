import { defineConfig } from "vitest/config";

// Pure logic and Vue reactivity tests use no browser or desktop runtime.
export default defineConfig({
  test: {
    environment: "node",
    include: ["tests/unit/**/*.test.ts"],
    maxWorkers: 2,
    restoreMocks: true,
  },
});
