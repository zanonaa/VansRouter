#!/usr/bin/env node
// Repeatable Lighthouse audit for the running app.
//
//   node scripts/lighthouse-audit.mjs                        # defaults below
//   node scripts/lighthouse-audit.mjs /landing /login        # specific paths
//   LH_BASE=http://127.0.0.1:3003 LH_RUNS=3 node scripts/lighthouse-audit.mjs
//
// Dashboard pages sit behind the login guard, so they 307 to /masuk and score as
// a redirect. To audit them, export a dashboard session cookie (browser devtools
// → Application → Cookies → the JWT the app sets after login):
//
//   LH_COOKIE='<name>=<value>' node scripts/lighthouse-audit.mjs /dashboard/usage
//
// Lighthouse runs the mobile profile (4x CPU throttle, slow 4G) by default —
// that is the profile the perf score is defined against. Scores are reported as
// the median of LH_RUNS runs because a single run on a shared machine is noisy.
"use strict";

import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const BASE = process.env.LH_BASE || "http://127.0.0.1:3003";
const RUNS = Math.max(1, Number(process.env.LH_RUNS || 3));
const COOKIE = process.env.LH_COOKIE || "";
const PATHS = process.argv.slice(2).length
  ? process.argv.slice(2)
  : ["/landing", "/login"];

const median = (xs) => [...xs].sort((a, b) => a - b)[Math.floor(xs.length / 2)];

function runOnce(url, outDir, i) {
  const out = path.join(outDir, `lh-${i}.json`);
  const args = [
    "--yes", "lighthouse@12", url,
    "--only-categories=performance,accessibility,best-practices,seo",
    "--chrome-flags=--headless=new --no-sandbox --disable-dev-shm-usage",
    "--output=json", `--output-path=${out}`, "--quiet",
  ];
  const extra = COOKIE ? ["--extra-headers", JSON.stringify({ Cookie: COOKIE })] : [];
  execFileSync("npx", [...args.slice(0, 2), ...extra, ...args.slice(2)], {
    stdio: ["ignore", "ignore", "ignore"],
  });
  return JSON.parse(readFileSync(out, "utf8"));
}

function summarise(run) {
  const perf = run.categories.performance;
  const failing = perf.auditRefs
    .filter((ref) => ref.weight > 0 && run.audits[ref.id]?.score !== null && run.audits[ref.id]?.score < 1)
    .map((ref) => `${ref.id}=${run.audits[ref.id].displayValue || run.audits[ref.id].score}`);
  return {
    perf: Math.round(perf.score * 100),
    a11y: Math.round(run.categories.accessibility.score * 100),
    bp: Math.round(run.categories["best-practices"].score * 100),
    seo: Math.round(run.categories.seo.score * 100),
    lcp: Math.round(run.audits.metrics.details.items[0].largestContentfulPaint),
    tbt: Math.round(run.audits.metrics.details.items[0].totalBlockingTime),
    // The trace's own LCP, before lantern re-estimates it under throttling.
    measuredLcp: Math.round(run.audits.metrics.details.items[0].observedLargestContentfulPaint || 0),
    blockingCssMs: Math.round(
      (run.audits["render-blocking-resources"]?.details?.items || [])
        .reduce((s, x) => s + (x.wastedMs || 0), 0),
    ),
    totalJsKb: Math.round(
      (run.audits["network-requests"]?.details?.items || [])
        .filter((x) => x.resourceType === "Script")
        .reduce((s, x) => s + (x.transferSize || 0), 0) / 1024,
    ),
    failing,
  };
}

const dir = mkdtempSync(path.join(tmpdir(), "lh-audit-"));
const rows = [];
try {
  for (const p of PATHS) {
    const url = p.startsWith("http") ? p : BASE + p;
    const runs = [];
    for (let i = 0; i < RUNS; i++) runs.push(summarise(runOnce(url, dir, `${p.replace(/\W+/g, "_")}-${i}`)));
    rows.push({
      path: p,
      perf: median(runs.map((r) => r.perf)),
      a11y: median(runs.map((r) => r.a11y)),
      bp: median(runs.map((r) => r.bp)),
      seo: median(runs.map((r) => r.seo)),
      lcp: median(runs.map((r) => r.lcp)),
      measuredLcp: median(runs.map((r) => r.measuredLcp)),
      tbt: median(runs.map((r) => r.tbt)),
      blockingCssMs: median(runs.map((r) => r.blockingCssMs)),
      jsKb: median(runs.map((r) => r.totalJsKb)),
      failing: runs[0].failing,
    });
  }
} finally {
  rmSync(dir, { recursive: true, force: true });
}

const pad = (v, n) => String(v).padStart(n);
console.log(`\nLighthouse (mobile profile, median of ${RUNS}):  ${BASE}\n`);
console.log(
  pad("page", 26) + pad("perf", 6) + pad("a11y", 6) + pad("bp", 5) + pad("seo", 5) +
  pad("LCP", 8) + pad("traceLCP", 10) + pad("TBT", 7) + pad("blkCSS", 8) + pad("JS", 8),
);
console.log("-".repeat(90));
for (const r of rows) {
  console.log(
    pad(r.path, 26) + pad(r.perf, 6) + pad(r.a11y, 6) + pad(r.bp, 5) + pad(r.seo, 5) +
    pad(r.lcp + "ms", 8) + pad(r.measuredLcp + "ms", 10) + pad(r.tbt + "ms", 7) +
    pad(r.blockingCssMs + "ms", 8) + pad(r.jsKb + "KB", 8),
  );
}
console.log("\nfailing perf audits:");
for (const r of rows) console.log(`  ${r.path}: ${r.failing.join(", ") || "(none)"}`);
if (!COOKIE && rows.some((r) => r.perf < 50)) {
  console.log("\nnote: a very low score with a 307 usually means the page redirected to /masuk. See LH_COOKIE above.");
}
