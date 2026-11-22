import { defineConfig } from 'astro/config';
export default defineConfig({
  site: process.env.WEBSITE_SITE || 'https://zjing1125.github.io',
  base: process.env.WEBSITE_BASE || '/LeaguePingo/',
  outDir: process.env.WEBSITE_OUT_DIR || './dist',
  output: 'static',
  build: { format: 'directory' },
});
