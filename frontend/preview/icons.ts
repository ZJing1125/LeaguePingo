import carbon from "@iconify-json/carbon/icons.json";

// Histoire bundles its own Iconify runtime. Its supported preload global avoids
// a second Iconify instance and keeps workbench navigation icons offline too.
if (typeof window !== "undefined") {
  const target = window as Window & { IconifyPreload?: unknown };
  const previous = target.IconifyPreload;
  target.IconifyPreload = [
    ...(Array.isArray(previous) ? previous : previous ? [previous] : []),
    carbon,
  ];
}
