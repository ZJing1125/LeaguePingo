import { defineConfig } from 'astro/config';
export default defineConfig({
  site: process.env.WEBSITE_SITE || 'https://zjing1125.github.io',
  base: process.env.WEBSITE_BASE || '/LeaguePingo/',
  output: 'static',
  build: { format: 'directory' },
});
